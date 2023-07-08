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
	GoTestArgs     []string
	Workers        int
}

func (opts *Options) Validate() error {
	ec := &erc.Collector{}

	erc.When(ec, opts.Workers < 1, "gadget options cannot specify 0 or fewer workers")
	erc.When(ec, opts.Timeout < time.Second, "timeout should be at least a second")
	erc.Whenf(ec, strings.HasSuffix(opts.RootPath, "..."), "root path [%s] should not have ...", opts.RootPath)

	if ec.HasErrors() {
		return ec.Resolve()
	}

	if strings.HasPrefix(opts.PackagePath, "./") {
		switch len(opts.PackagePath) {
		case 2:
			opts.PackagePath = ""
		default:
			opts.PackagePath = opts.PackagePath[3:]
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

func strptr(in string) *string { return &in }

func RunTests(ctx context.Context, opts Options) error {
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
			return strptr(filepath.Dir(path)), nil
		}

		// for non-recursive cases, abort early.
		return nil, fs.SkipAll
	})
	jpm := jasper.Context(ctx)
	ec := &erc.Collector{}

	main := pubsub.NewUnlimitedQueue[fun.Worker]()
	pool := srv.WorkerPool(main,
		fun.WorkerGroupConfSet(&fun.WorkerGroupConf{NumWorkers: opts.Workers, ContinueOnError: true}),
	)
	if err := pool.Start(ctx); err != nil {
		return err
	}

	out := send.MakeWriter(send.MakePlain())
	out.SetPriority(grip.Sender().Priority())

	count := 0
	for iter.Next(ctx) {
		modulePath := iter.Value()
		if count != 0 && !opts.Recursive {
			grip.Infof("module at %q is within %q but recursive mode is not enabled",
				modulePath, opts.RootPath)
			ec.Add(iter.Close())
			break
		}
		count++

		shortName := filepath.Base(modulePath)
		pkgs, err := Collect(ctx, modulePath)
		if err != nil {
			ec.Add(err)
			continue
		}

		pkgidx := pkgs.IndexByPackageName()
		mod := pkgs.IndexByLocalDirectory()[modulePath]

		grip.Build().Level(level.Debug).
			Pair("pkg", mod.ModuleName).
			Pair("op", "populate").
			Pair("pkgs", len(pkgs)).
			Pair("path", util.TryCollapseHomedir(modulePath)).
			Send()

		start := time.Now()
		allReports := &adt.Map[string, testReport]{}
		ec.Add(main.Add(func(ctx context.Context) error {
			reports := &adt.Map[string, testReport]{}
			catch := &erc.Collector{}
			var err error
			if !opts.CompileOnly {
				lintStart := time.Now()
				err := jpm.CreateCommand(ctx).
					ID(fmt.Sprint("lint.", shortName)).
					Directory(modulePath).
					SetOutputSender(level.Debug, out).
					SetErrorSender(level.Error, out).
					PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
					Append(fmt.Sprint("golangci-lint run --allow-parallel-runners ", modulePath)).
					Run(ctx)
				lintDur := time.Since(lintStart)
				catch.Add(erc.Wrapf(err, "lint errors for %q", modulePath))

				grip.Build().Level(level.Info).
					Pair("pkg", mod.ModuleName).
					Pair("op", "lint").
					Pair("ok", err == nil).
					Pair("dur", lintDur).
					Send()
			}

			sender := &bufsend{
				buffer: &bytes.Buffer{},
			}
			sender.SetPriority(level.Info)
			sender.SetName("coverage.report")
			sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

			testOut := send.MakeWriter(send.MakePlain())
			testOut.SetPriority(grip.Sender().Priority())
			testOut.SetFormatter(testOutputFilter(opts, reports, pkgidx))
			testOut.SetErrorHandler(func(err error, m message.Composer) {
				grip.ErrorWhen(!errors.Is(err, io.EOF), message.WrapError(err, m))
			})
			args := []string{"go", "test", "-race", "-parallel=8", fmt.Sprint("-timeout=", opts.Timeout)}
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
				Directory(modulePath).
				SetOutputSender(level.Debug, testOut).
				SetErrorSender(level.Error, testOut).
				PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
				Bash("go mod tidy || true").
				Add(args).
				Run(ctx)
			testDur := time.Since(testStart)

			grip.Build().Level(level.Info).
				Pair("pkg", mod.ModuleName).
				Pair("op", "test").
				Pair("ok", err == nil).
				Pair("dur", testDur).
				Send()

			catch.Add(err)

			ec.Add(reports.Iterator().Observe(ctx, func(tr dt.Pair[string, testReport]) { allReports.Set(tr) }))

			if err == nil && opts.ReportCoverage {
				grip.Warning(erc.Wrapf(jpm.CreateCommand(ctx).
					ID(fmt.Sprint("coverage.html.", shortName)).
					Directory(modulePath).
					PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
					SetOutputSender(level.Debug, out).
					SetErrorSender(level.Error, out).
					Append("go tool cover -html=coverage.out -o coverage.html").
					Run(ctx), "coverage html for %s", shortName))

				catch.Add(jpm.CreateCommand(ctx).
					ID(fmt.Sprint("coverage.report", modulePath)).
					Directory(modulePath).
					SetErrorSender(level.Error, out).
					SetOutputSender(level.Info, sender).
					Append("go tool cover -func=coverage.out").
					Run(ctx))
			}

			dur := time.Since(start)
			report(ctx, mod, reports.Get(mod.PackageName), strings.TrimSpace(sender.buffer.String()), dur, catch.Resolve())
			ec.Add(reports.Values().Observe(ctx, func(tr testReport) { grip.Sender().Send(tr.Message()) }))
			return catch.Resolve()
		}))
	}

	ec.Add(main.Close())
	ec.Add(pool.Wait())

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
	MissingTests    bool
}

func (tr testReport) Message() message.Composer {
	var priority level.Priority

	pairs := &dt.Pairs[string, any]{}
	pairs.Add("pkg", tr.Package)

	if tr.MissingTests {
		priority = level.Warning
		pairs.Add("state", "untested")
	} else {
		priority = level.Info
		if tr.Cached {
			pairs.Add("cached", true)
		} else if tr.CoverageEnabled {
			pairs.Add("coverage", tr.Coverage)
		} else if !tr.CompileOnly {
			pairs.Add("dur", tr.Duration)
		}
	}

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
			}
			mp.Store(report.Package, report)
			return "", io.EOF
		} else if strings.HasPrefix(mstr, "?") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:         parts[1],
				Info:            pkgs[parts[1]],
				MissingTests:    true,
				CompileOnly:     opts.CompileOnly,
				CoverageEnabled: opts.ReportCoverage,
			}
			mp.Store(report.Package, report)
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

	iter := LineIterator(strings.NewReader(coverage))
	table := tabby.New()
	replacer := strings.NewReplacer(mod.ModuleName, pfx)
	err = erc.Join(
		err,
		iter.Observe(ctx, func(in string) {
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
		}))

	pairs := &dt.Pairs[string, any]{}

	pairs.Add("pkg", mod.PackageName)

	if mod.PackageName != mod.ModuleName {
		pairs.Add("mod", mod.ModuleName)
	}

	pairs.Add("dur", tr.Duration)
	pairs.Add("path", mod.LocalDirectory)

	if tr.MissingTests {
		pairs.Add("state", "no tests")
	} else if tr.Cached {
		pairs.Add("state", "cached")
	} else if tr.CoverageEnabled {
		pairs.Add("coverage", tr.Coverage)
		pairs.Add("funcs", count)

		if numUncovered != 0 {
			pairs.Add("uncovered", numUncovered)
		}
		if numCovered != count {
			pairs.Add("covered", numCovered)
		}
	}

	if err != nil {
		pairs.Add("err", err)
	}

	msg := message.MakePairs(pairs)
	switch {
	case tr.MissingTests:
		msg.SetPriority(level.Warning)
	case err != nil:
		msg.SetPriority(level.Error)
	case tr.Cached:
		msg.SetPriority(level.Info)
	default:
		msg.SetPriority(level.Info)
	}
	grip.Sender().Send(msg)

	table.Print()

}
