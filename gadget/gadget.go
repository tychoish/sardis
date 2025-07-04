package gadget

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/risky"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/libfun"
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
	defer func() { err = erc.ParsePanic(recover()) }()

	ec := &erc.Collector{}

	ec.When(opts.Workers < 1, ers.Error("gadget options cannot specify 0 or fewer workers"))
	ec.When(opts.Timeout < time.Second, ers.Error("timeout should be at least a second"))
	ec.Whenf(strings.HasSuffix(opts.RootPath, "..."), "root path [%s] should not have ...", opts.RootPath)

	ec.Whenf(opts.Recursive && (opts.PackagePath != "" && opts.PackagePath != "./" && opts.PackagePath != "..."),
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
	iter := libfun.WalkDirIterator(opts.RootPath, func(path string, d fs.DirEntry) (*string, error) {
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
			startLint := time.Now()
			err := jpm.CreateCommand(ctx).
				ID(fmt.Sprint("lint.", name)).
				Directory(opts.RootPath).
				AddEnv("SARDIS_LOG_QUIET_SYSLOG", "true").
				SetOutputSender(level.Info, out).
				SetErrorSender(level.Error, out).
				AppendArgs("golangci-lint", "run", "--allow-parallel-runners", "--timeout", opts.Timeout.String()).
				Run(ctx)

			dur := time.Since(startLint)

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
	mods := &adt.Map[string, *Module]{}
	moditer := fun.Converter[string, *Module](func(ctx context.Context, mpath string) (*Module, error) {
		if count != 0 && !opts.Recursive {
			return nil, fmt.Errorf("module at %q is within %q but recursive mode is not enabled",
				mpath, opts.RootPath)
		}
		count++
		mod, err := Collect(ctx, mpath)
		mods.Store(mpath, mod)
		return mod, err
	}).Stream(iter)

	if opts.Recursive {
		moditer = moditer.BufferParallel(max(4, max(1, opts.Workers/2)))
	}

	reports := &adt.Map[string, testReport]{}

	pkgiter := fun.MergeStreams(fun.MakeConverter(func(m *Module) *fun.Stream[PackageInfo] { return fun.SliceStream(m.Packages) }).Stream(moditer))

	fun.MakeConverter(func(pkg PackageInfo) fun.Worker {
		return func(ctx context.Context) error {
			if reports.Check(pkg.PackageName) {
				grip.Errorln("duplicate", pkg.PackageName)
				return nil
			}
			grip.Build().Level(level.Debug).
				Pair("pkg", pkg.PackageName).
				Pair("op", "populate").
				Pair("mod", pkg.ModuleName).
				Pair("path", pkg.LocalDirectory).
				Send()

			catch := &erc.Collector{}

			testOut := send.MakeWriter(send.MakePlain())
			testOut.SetPriority(grip.Sender().Priority())
			testOut.SetFormatter(testOutputFilter(opts, reports, pkg))
			testOut.SetErrorHandler(func(err error) { grip.ErrorWhen(!errors.Is(err, io.EOF), err) })
			args := []string{
				"go", "test", "-race",
				fmt.Sprintf("-parallel=%d", min(4, opts.Workers/2)),
				fmt.Sprintf("-timeout=%s", opts.Timeout),
			}

			switch {
			case opts.CompileOnly:
				args = append(args, "-run=noop")
			case opts.ReportCoverage:
				args = append(args, fmt.Sprintf("-coverprofile=%s", filepath.Join(pkg.LocalDirectory, "coverage.out")))
			case opts.UseCache:
				// nothing
			default:
				return fmt.Errorf("must configure coverage, cache, or compile-only")
			}

			args = append(args, pkg.LocalDirectory)
			args = append(args, opts.GoTestArgs...)

			testStart := time.Now()
			catch.Add(jpm.CreateCommand(ctx).
				ID(fmt.Sprint("test.", pkg.PackageName)).
				Directory(pkg.LocalDirectory).
				AddEnv("SARDIS_LOG_QUIET_SYSLOG", "true").
				SetOutputSender(level.Info, testOut).
				SetErrorSender(level.Error, testOut).
				PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
				Add(args).
				Run(ctx))
			dur := time.Since(testStart)

			grip.Build().Level(level.Debug).
				Pair("pkg", pkg.PackageName).
				Pair("op", "test").
				Pair("ok", catch.Ok()).
				Pair("dur", dur).
				Send()

			if result, ok := reports.Load(pkg.PackageName); ok {
				var buf bytes.Buffer
				sender := send.MakeBytesBuffer(&buf)
				sender.SetPriority(level.Info)
				sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

				if opts.ReportCoverage {
					sender.SetName("coverage.report")
					coverout := filepath.Join(result.Info.LocalDirectory, "coverage.out")
					grip.Warning(ers.Wrapf(jpm.CreateCommand(ctx).
						ID(fmt.Sprint("coverage.html.", result.Package)).
						Directory(result.Info.LocalDirectory).
						PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
						SetOutputSender(level.Debug, out).
						SetErrorSender(level.Error, out).
						AddEnv("SARDIS_LOG_QUIET_SYSLOG", "true").
						AppendArgs("go", "tool", "cover", "-html", coverout, "-o", fmt.Sprintf("coverage-%s.html", filepath.Base(result.Info.LocalDirectory))).
						Run(ctx), "coverage html for %s", result.Package))
					ec.Add(jpm.CreateCommand(ctx).
						ID(fmt.Sprint("coverage.report", result.Package)).
						Directory(result.Info.LocalDirectory).
						AddEnv("SARDIS_LOG_QUIET_SYSLOG", "true").
						SetErrorSender(level.Error, out).
						SetOutputSender(level.Info, sender).
						AppendArgs("go", "tool", "cover", "-func", coverout).
						Run(ctx))
				}
				report(ctx, result.Info, reports.Get(result.Info.PackageName), strings.TrimSpace(buf.String()), time.Since(start), catch.Resolve())
			}

			return catch.Resolve()
		}
	}).Stream(pkgiter).ReadAll(fun.MakeHandler(main.Add).Capture()).Operation(ec.Push).Run(ctx)

	ec.Add(main.Close())
	ec.Add(pool.Worker().Run(ctx))

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
	pairs.Add("dur", tr.Duration.Round(time.Millisecond))

	out := message.MakePairs(pairs)
	out.SetPriority(priority)

	return out
}

func testOutputFilter(
	opts Options,
	mp *adt.Map[string, testReport],
	info PackageInfo,
) send.MessageFormatter {
	return func(m message.Composer) (_ string, err error) {
		defer func() {
			ec := &erc.Collector{}
			ec.Add(err)
			ec.Recover()
			err = ec.Resolve()
		}()

		mstr := m.String()
		if strings.HasPrefix(mstr, "ok") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:         parts[1],
				Cached:          strings.Contains(mstr, "cached"),
				Info:            info,
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
			grip.WarningWhen(mp.Check(report.Package), message.MakeLines("DUPE", report.Package))
			mp.Store(report.Package, report)
			grip.Debug(func() message.Composer {
				m := report.Message()
				m.Annotate("src", "filter")
				m.Annotate("case", "pass")
				return m
			})
			return "", io.EOF
		} else if strings.HasPrefix(mstr, "?") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:         parts[1],
				Info:            info,
				Cached:          strings.Contains(mstr, "cached"),
				MissingTests:    true,
				CompileOnly:     opts.CompileOnly,
				CoverageEnabled: opts.ReportCoverage,
			}
			mp.Store(report.Package, report)
			grip.Debug(func() message.Composer {
				m := report.Message()
				m.Annotate("src", "filter")
				m.Annotate("case", "failure")
				return m
			})
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

	iter := fun.MAKE.Lines(strings.NewReader(coverage))
	table := tabby.New()
	replacer := strings.NewReplacer(tr.Package, pfx)
	err = erc.Join(err, iter.ReadAll(func(in string) {

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

		cols[0] = strings.TrimPrefix(cols[0], "/")

		table.AddLine(cols[0], cols[1], cols[2])
	}).Run(ctx))

	msg := grip.Build().PairBuilder()
	msg.Pair("pkg", mod.PackageName)

	if tr.MissingTests {
		msg.Pair("state", "no tests")
	} else if tr.Cached {
		msg.Pair("state", "cached")
	}

	if tr.CoverageEnabled {
		if tr.Coverage == 100.0 {
			msg.Pair("covered", true)
		} else {
			msg.Pair("coverage", tr.Coverage)
			msg.Pair("fncount", count)
			msg.Pair("covered", numCovered)
		}
		table.Print()
	}

	msg.Pair("wal", runtime.Round(time.Millisecond))
	msg.Pair("dur", tr.Duration.Round(time.Millisecond))

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
	msg.Send()
}
