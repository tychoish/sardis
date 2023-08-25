package gadget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/risky"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/sardis/util"
)

type Options struct {
	Notify         string
	RootPath       string
	PackagePath    string
	Timeout        time.Duration
	Recursive      bool
	CompileOnly    bool
	UseCache       bool
	ReportCoverage bool
	SkipLint       bool
	GoTestArgs     []string
	Workers        int
}

func (opts *Options) Validate() (err error) {
	defer func() { err = ers.ParsePanic(recover()) }()

	ec := &erc.Collector{}

	erc.When(ec, opts.Workers < 1, "gadget options cannot specify 0 or fewer workers")
	erc.When(ec, opts.Timeout < time.Second, "timeout should be at least a second")
	erc.Whenf(ec, strings.HasSuffix(opts.RootPath, "..."), "root path [%s] should not have ...", opts.RootPath)

	erc.Whenf(ec, opts.Recursive && (opts.PackagePath != "" && opts.PackagePath != "./" && opts.PackagePath != "..."),
		"recursive gadget calls must not specify a limiting path %q", opts.PackagePath)

	fun.Invariant.Must(ec.Resolve())

	if strings.HasPrefix(opts.PackagePath, "./") {
		switch len(opts.PackagePath) {
		case 2:
			opts.PackagePath = ""
		default:
			opts.PackagePath = opts.PackagePath[2:]
		}
	}

	if opts.PackagePath == "" {
		opts.PackagePath = "..."
	}

	if opts.Recursive && !strings.HasSuffix(opts.PackagePath, "...") {
		opts.PackagePath = filepath.Join(opts.PackagePath, "...")
	}

	if !filepath.IsAbs(opts.RootPath) {
		opts.RootPath = risky.Try(filepath.Abs, opts.RootPath)
	}

	if opts.ReportCoverage {
		opts.CompileOnly = false
	}

	return nil
}

func RunTests(ctx context.Context, opts Options) error {
	start := time.Now()
	defer func() {
		grip.Build().Level(level.Info).
			Pair("app", "gadget").
			Pair("op", "run-tests").
			Pair("workers", opts.Workers).
			Pair("dur", time.Since(start)).
			Send()
	}()

	var seenOne bool
	iter := WalkDirIterator(ctx, opts.RootPath, func(path string, d fs.DirEntry) (*string, error) {
		name := d.Name()
		if d.Type().IsDir() && name == ".git" {
			return nil, fs.SkipDir
		}

		if name != "go.mod" {
			return nil, nil
		}
		if !seenOne || seenOne && opts.Recursive {
			seenOne = true
			return ft.Ptr(filepath.Dir(path)), nil
		}

		// for non-recursive cases, abort early.
		return nil, fs.SkipAll
	})
	jpm := jasper.Context(ctx)
	ec := &erc.Collector{}

	main := pubsub.NewUnlimitedQueue[fun.Worker]()
	pool := srv.WorkerPool(main, fun.WorkerGroupConfSet(&fun.WorkerGroupConf{
		NumWorkers:      opts.Workers,
		ContinueOnError: true,
	}))
	if err := pool.Start(ctx); err != nil {
		return err
	}

	out := send.MakeWriter(send.MakePlain())
	out.SetPriority(level.Debug)
	out.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

	if !opts.CompileOnly && !opts.SkipLint {
		ec.Add(main.Add(func(ctx context.Context) error {
			name := filepath.Base(opts.RootPath)
			start := time.Now()
			err := jpm.CreateCommand(ctx).
				ID(fmt.Sprint("lint.", name)).
				Directory(opts.RootPath).
				SetOutputSender(level.Info, out).
				SetErrorSender(level.Error, out).
				AppendArgs("golangci-lint", "run", "--allow-parallel-runners").
				Run(ctx)

			dur := time.Since(start)

			grip.Build().Level(level.Info).
				Pair("project", name).
				Pair("op", "lint").
				Pair("ok", err == nil).
				Pair("dur", dur).
				Send()

			return ers.Wrapf(err, "lint errors for %q", opts.RootPath)
		}))
	}

	count := 0

	pkgiter := fun.ConvertIterator(iter, func(ctx context.Context, mpath string) (*Module, error) {
		if count != 0 && !opts.Recursive {
			return nil, fmt.Errorf("module at %q is within %q but recursive mode is not enabled",
				mpath, opts.RootPath)
		}
		count++

		return Collect(ctx, mpath)
	})

	if opts.Recursive {
		pkgiter = pkgiter.ParallelBuffer(intish.Max(4, opts.Workers/2))
	}

	fun.ConvertIterator(
		pkgiter,
		fun.Converter(func(module *Module) fun.Worker {
			return func(ctx context.Context) error {
				pkgs := module.Packages
				pkgidx := pkgs.IndexByPackageName()
				mod := pkgs.IndexByLocalDirectory()[module.Path]
				shortName := filepath.Base(module.Path)

				grip.Build().Level(level.Debug).
					Pair("pkg", mod.ModuleName).
					Pair("op", "populate").
					Pair("pkgs", len(pkgs)).
					Pair("path", util.TryCollapseHomedir(module.Path)).
					Send()

				start := time.Now()
				reports := &adt.Map[string, testReport]{}

				catch := &erc.Collector{}
				var err error

				testOut := send.MakeWriter(send.MakePlain())
				testOut.SetPriority(grip.Sender().Priority())
				testOut.SetFormatter(testOutputFilter(opts, reports, pkgidx))
				testOut.SetErrorHandler(func(err error) { grip.ErrorWhen(!errors.Is(err, io.EOF), err) })
				args := []string{
					"go", "test", "-race",
					fmt.Sprintf("-parallel=%d", runtime.NumCPU()),
					fmt.Sprint("-timeout=", opts.Timeout),
				}

				switch {
				case opts.CompileOnly:
					args = append(args, "-run=noop", fmt.Sprint("./", opts.PackagePath))
				case opts.ReportCoverage:
					args = append(args, "-coverprofile=coverage.out")
				case opts.UseCache:
					// nothing
				default:
					return fmt.Errorf("must configure coverage, cache, or compile-only")
				}

				args = append(args, fmt.Sprint("./", opts.PackagePath))
				args = append(args, opts.GoTestArgs...)

				testStart := time.Now()
				err = jpm.CreateCommand(ctx).
					ID(fmt.Sprint("test.", shortName)).
					Directory(module.Path).
					SetOutputSender(level.Debug, testOut).
					SetErrorSender(level.Error, testOut).
					PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
					Add(args).
					Run(ctx)
				dur := time.Since(testStart)

				grip.Build().Level(level.Info).
					Pair("project", filepath.Base(opts.RootPath)).
					Pair("op", "test").
					Pair("ok", err == nil).
					Pair("dur", dur).
					Send()

				catch.Add(err)

				sender := &bufsend{}
				if err == nil && opts.ReportCoverage {
					sender.SetPriority(level.Info)
					sender.SetName("coverage.report")
					sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

					grip.Warning(ers.Wrapf(jpm.CreateCommand(ctx).
						ID(fmt.Sprint("coverage.html.", shortName)).
						Directory(module.Path).
						PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
						SetOutputSender(level.Debug, out).
						SetErrorSender(level.Error, out).
						Append("go tool cover -html=coverage.out -o coverage.html").
						Run(ctx), "coverage html for %s", shortName))

					catch.Add(jpm.CreateCommand(ctx).
						ID(fmt.Sprint("coverage.report", module.Path)).
						Directory(module.Path).
						SetErrorSender(level.Error, out).
						SetOutputSender(level.Info, sender).
						Append("go tool cover -func=coverage.out").
						Run(ctx))
				}

				report(ctx, mod, reports.Get(mod.PackageName), strings.TrimSpace(sender.buffer.String()), time.Since(start), catch.Resolve())
				return catch.Resolve()
			}
		})).
		Process(fun.MakeProcessor(main.Add)).
		PostHook(func() { ec.Add(main.Close()); ec.Add(pool.Wait()) }).Operation(ec.Add).Run(ctx)

	return ec.Resolve()
}

type Percent float64

func (p Percent) String() string { return fmt.Sprintf("%.1f%%", p) }

type testReport struct {
	Package         string
	Coverage        Percent
	Duration        time.Duration
	Info            PackageInfo
	CompileOnly     bool
	CoverageEnabled bool
	Cached          bool
	FullyCovered    bool
	MissingTests    bool
}

func (tr testReport) Message() message.Composer {
	var priority level.Priority

	// TODO: could move coverage or duration (plus padding?) to
	// the beginning of the line so it works better as columns
	pairs := &dt.Pairs[string, any]{}
	pairs.Add("pkg", tr.Package)
	if tr.MissingTests {
		priority = level.Warning
		pairs.Add("state", "untested")
	} else {
		priority = level.Info
		if tr.Cached {
			pairs.Add("cached", true)
		}
		if tr.CoverageEnabled {
			if tr.FullyCovered {
				pairs.Add("covered", true)
			} else {
				pairs.Add("covered", tr.Coverage)
			}
		}
	}
	pairs.Add("dur", tr.Duration)

	out := message.MakePairs(pairs)
	out.SetPriority(priority)

	return out
}

func testOutputFilter(
	opts Options,
	mp *adt.Map[string, testReport],
	pkgs map[string]PackageInfo,
) send.MessageFormatter {
	return func(m message.Composer) (_ string, err error) {
		defer func() {
			ec := &erc.Collector{}
			ec.Add(err)
			erc.Recover(ec)
			err = ec.Resolve()
		}()

		mstr := m.String()
		if strings.HasPrefix(mstr, "ok") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:         parts[1],
				Cached:          strings.Contains(mstr, "cached"),
				Info:            pkgs[parts[1]],
				CompileOnly:     opts.CompileOnly,
				CoverageEnabled: opts.ReportCoverage,
			}
			if !report.Cached {
				report.Duration = ft.Must(time.ParseDuration(parts[2]))
			}
			if opts.ReportCoverage {
				report.Coverage = Percent(ft.Must(strconv.ParseFloat(parts[4][:len(parts[4])-1], 64)))
				if report.Coverage == 100 {
					report.FullyCovered = true
				}
			}
			mp.Store(report.Package, report)
			grip.Info(report.Message)
			return "", io.EOF
		} else if strings.HasPrefix(mstr, "?") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:         parts[1],
				Info:            pkgs[parts[1]],
				Cached:          strings.Contains(mstr, "cached"),
				MissingTests:    true,
				CompileOnly:     opts.CompileOnly,
				CoverageEnabled: opts.ReportCoverage,
			}
			mp.Store(report.Package, report)
			grip.Info(report.Message)
			return "", io.EOF
		} else if strings.HasPrefix(mstr, "FAIL") {
			return "", io.EOF
		}

		// TODO full qualify lines that start with whitespace + line
		// number to be not line number.
		//
		// alternately see what json output does
		return mstr, nil
	}
}

func report(
	ctx context.Context,
	mod PackageInfo,
	tr testReport,
	coverage string,
	runtime time.Duration,
	err error,
) {
	pwd := ft.Must(os.Getwd())
	pfx := mod.LocalDirectory
	if pwd == pfx {
		pfx = ""
	} else if strings.HasPrefix(pfx, pwd) {
		pfx = pfx[len(pwd)+1:]
	}

	var (
		numCovered   int
		numUncovered int
	)
	count := 0

	iter := fun.HF.Lines(strings.NewReader(coverage))
	table := tabby.New()
	replacer := strings.NewReplacer(mod.ModuleName, pfx)
	err = ers.Join(err, iter.Observe(func(in string) {
		cols := strings.Fields(replacer.Replace(in))
		if cols[0] == "total:" {
			// we read this out of the go.test line
			return
		}
		count++

		if strings.HasSuffix(in, "100.0%") {
			numCovered++
			return
		}

		numUncovered++

		if strings.HasPrefix(cols[0], "/") {
			cols[0] = cols[0][1:]
		}
		table.AddLine(cols[0], cols[1], cols[2])
	}).Run(ctx))

	msg := grip.Build().PairBuilder()
	defer msg.Send()
	msg.Pair("mod", mod.ModuleName)

	if mod.PackageName != mod.ModuleName {
		msg.Pair("pkg", mod.PackageName)
	}

	msg.Pair("path", mod.LocalDirectory)

	if tr.MissingTests {
		msg.Pair("state", "no tests")
	} else if tr.Cached {
		msg.Pair("state", "cached")
	} else {
		msg.Pair("state", "exec")
	}
	msg.Pair("dur", tr.Duration)

	if tr.CoverageEnabled {
		msg.Pair("funcs", count)
		if tr.Coverage == 100.0 {
			msg.Pair("covered", true)
		} else {
			msg.Pair("covered", numCovered)
			msg.Pair("coverage", tr.Coverage)
		}
		table.Print()
	}

	switch {
	case err != nil:
		msg.Pair("err", err)
		msg.SetPriority(level.Error)
	case tr.MissingTests:
		msg.SetPriority(level.Warning)
	case tr.Cached:
		msg.SetPriority(level.Info)
	default:
		msg.SetPriority(level.Info)
	}
}
