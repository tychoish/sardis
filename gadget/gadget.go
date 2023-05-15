package gadget

// TODO: use KV rather than fields for easier to parse order by eyes

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
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/daggen"
)

type Options struct {
	Notify     string
	RootPath   string
	Path       string
	Timeout    time.Duration
	Recursive  bool
	GoTestArgs []string
}

func (opts *Options) Validate() error {
	if opts.Path == "" {
		opts.Path = "..."
	}
	return nil
}

func strptr(in string) *string { return &in }

func RunTests(ctx context.Context, opts Options) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	iter := WalkDirIterator(ctx, opts.RootPath, func(path string, d fs.DirEntry) (*string, error) {
		if d.Name() != "go.mod" {
			return nil, nil
		}

		return strptr(filepath.Dir(path)), nil
	})
	jpm := jasper.Context(ctx)
	ec := &erc.Collector{}

	main := pubsub.NewUnlimitedQueue[fun.WorkerFunc]()
	pool := srv.WorkerPool(main, itertool.Options{NumWorkers: 4, ContinueOnError: true})
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
		pkgs, err := daggen.Collect(ctx, modulePath)
		if err != nil {
			ec.Add(err)
			continue
		}

		pkgidx := pkgs.IndexByPackageName()
		mod := pkgs.IndexByLocalDirectory()[modulePath]

		grip.Build().Level(level.Debug).
			SetOption(message.OptionSkipAllMetadata).
			Pair("pkg", mod.ModuleName).
			Pair("op", "populate").
			Pair("pkgs", len(pkgs)).
			Pair("path", util.CollapseHomedir(modulePath)).
			Send()

		start := time.Now()
		allReports := &adt.Map[string, testReport]{}
		ec.Add(main.Add(func(ctx context.Context) error {
			reports := &adt.Map[string, testReport]{}

			catch := &erc.Collector{}
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
				SetOption(message.OptionSkipAllMetadata).
				Pair("pkg", mod.ModuleName).
				Pair("op", "lint").
				Pair("ok", err == nil).
				Pair("dur", lintDur).
				Send()

			sender := &bufsend{
				buffer: &bytes.Buffer{},
			}
			sender.SetPriority(level.Info)
			sender.SetName("coverage.report")
			sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

			testOut := send.MakeWriter(send.MakePlain())
			testOut.SetPriority(grip.Sender().Priority())
			testOut.SetFormatter(testOutputFilter(reports, pkgidx))
			testOut.SetErrorHandler(func(err error, m message.Composer) {
				grip.ErrorWhen(!errors.Is(err, io.EOF), message.WrapError(err, m))
			})

			args := []string{"go", "test", "-p=8", "-race", "-coverprofile=coverage.out", "--timeout", opts.Timeout.String(), fmt.Sprint("./", opts.Path)}
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
				SetOption(message.OptionSkipAllMetadata).
				Pair("pkg", mod.ModuleName).
				Pair("op", "test").
				Pair("ok", err == nil).
				Pair("dur", testDur).
				Send()

			catch.Add(err)

			if err == nil {
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

			ec.Add(fun.Observe(ctx, reports.Iterator(), func(tr fun.Pair[string, testReport]) { allReports.Set(tr) }))
			dur := time.Since(start)
			report(ctx, mod, reports.Get(mod.PackageName), strings.TrimSpace(sender.buffer.String()), dur, catch.Resolve())
			ec.Add(fun.Observe(ctx, reports.Values(), func(tr testReport) { grip.Sender().Send(tr.Message()) }))
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
	Package      string
	Coverage     Percent
	Duration     time.Duration
	Cached       bool
	MissingTests bool
	Info         daggen.PackageInfo
}

func (tr testReport) Message() message.Composer {
	var priority level.Priority

	pairs := fun.Pairs[string, any]{}
	pairs.Add("pkg", tr.Package)

	if tr.MissingTests {
		priority = level.Warning
		pairs.Add("state", "untested")
	} else {
		priority = level.Notice
		pairs.Add("coverage", tr.Coverage)
		pairs.Add("dur", tr.Duration)
		if tr.Cached {
			pairs.Add("cached", true)
		}
	}

	out := message.MakeKV(pairs...)
	out.SetPriority(priority)
	out.SetOption(message.OptionSkipAllMetadata)

	return out
}

func testOutputFilter(
	mp *adt.Map[string, testReport],
	pkgs map[string]daggen.PackageInfo,
) send.MessageFormatter {
	return func(m message.Composer) (_ string, err error) {
		defer func() { err = erc.Merge(err, erc.Recovery()) }()

		mstr := m.String()
		if strings.HasPrefix(mstr, "ok") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:  parts[1],
				Cached:   strings.Contains(mstr, "cached"),
				Duration: fun.Must(time.ParseDuration(parts[2])),
				Coverage: Percent(fun.Must(strconv.ParseFloat(parts[4][:len(parts[4])-1], 64))),
				Info:     pkgs[parts[1]],
			}
			mp.Store(report.Package, report)
			return "", io.EOF
		} else if strings.HasPrefix(mstr, "?") {
			parts := strings.Fields(mstr)

			report := testReport{
				Package:      parts[1],
				Info:         pkgs[parts[1]],
				MissingTests: true,
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
	mod daggen.PackageInfo,
	tr testReport,
	coverage string,
	runtime time.Duration,
	err error,
) {
	pwd := fun.Must(os.Getwd())
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

	iter := LineIterator(coverage)
	table := tabby.New()

	replacer := strings.NewReplacer(mod.ModuleName, pfx)
	err = erc.Merge(
		err,
		fun.Observe(ctx, iter, func(in string) {
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

	pairs := fun.MakePairs[string, any]()

	pairs.Add("pkg", mod.PackageName)

	if mod.PackageName != mod.ModuleName {
		pairs.Add("mod", mod.ModuleName)
	}

	pairs.Add("dur", tr.Duration)
	pairs.Add("path", mod.LocalDirectory)
	pairs.Add("coverage", tr.Coverage)
	pairs.Add("funcs", count)

	if numUncovered != 0 {
		pairs.Add("uncovered", numUncovered)
	}
	if numCovered != count {
		pairs.Add("covered", numCovered)
	}

	if tr.MissingTests {
		pairs.Add("state", "no tests")
	} else if tr.Cached {
		pairs.Add("state", "cached")
	}

	if err != nil {
		pairs.Add("err", err)
	}

	msg := message.MakeKV(pairs...)
	msg.SetOption(message.OptionSkipAllMetadata)
	switch {
	case tr.MissingTests:
		msg.SetPriority(level.Warning)
	case tr.Cached:
		msg.SetPriority(level.Info)
	case err != nil:
		msg.SetPriority(level.Error)
	default:
		msg.SetPriority(level.Notice)
	}

	table.Print()
}
