package gadget

import (
	"bufio"
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

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/global"
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

	ec.If(opts.Workers < 1, ers.Error("gadget options cannot specify 0 or fewer workers"))
	ec.If(opts.Timeout < time.Second, ers.Error("timeout should be at least a second"))
	ec.Whenf(strings.HasSuffix(opts.RootPath, "..."), "root path [%s] should not have ...", opts.RootPath)

	ec.Whenf(opts.Recursive && (opts.PackagePath != "" && opts.PackagePath != "./" && opts.PackagePath != "..."),
		"recursive gadget calls must not specify a limiting path %q", opts.PackagePath)

	if err := ec.Resolve(); err != nil {
		return err
	}

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
		if absPath, err := filepath.Abs(opts.RootPath); err == nil {
			opts.RootPath = absPath
		}
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
			Fields(message.Fields{
				"app":     "gadget",
				"op":      "run-tests",
				"workers": opts.Workers,
				"dur":     time.Since(start),
			}).
			Send()
	}()

	var seenOne bool
	iter, closer := libfun.WalkDirIterator(opts.RootPath, func(path string, d fs.DirEntry) (*string, error) {
		name := d.Name()
		if d.Type().IsDir() && name == ".git" {
			return nil, fs.SkipDir
		}

		if name != "go.mod" {
			return nil, nil
		}
		if !seenOne || seenOne && opts.Recursive {
			seenOne = true
			return stw.Ptr(filepath.Dir(path)), nil
		}

		// for non-recursive cases, abort early.
		return nil, fs.SkipAll
	})
	defer func() { _ = closer() }()
	jpm := jasper.Context(ctx)
	ec := &erc.Collector{}

	main := pubsub.NewUnlimitedQueue[fnx.Worker]()
	pool := srv.WorkerPool(main, wpa.WorkerGroupConfSet(&wpa.WorkerGroupConf{
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
		ec.Push(main.Push(func(ctx context.Context) error {
			name := filepath.Base(opts.RootPath)
			startLint := time.Now()
			err := jpm.CreateCommand(ctx).
				ID(fmt.Sprint("lint.", name)).
				Directory(opts.RootPath).
				AddEnv(global.EnvVarSardisLogQuietSyslog, "true").
				SetOutputSender(level.Info, out).
				SetErrorSender(level.Error, out).
				AppendArgs("golangci-lint", "run", "--allow-parallel-runners", "--timeout", opts.Timeout.String()).
				Run(ctx)

			dur := time.Since(startLint)

			grip.Build().Level(level.Info).
				KV("project", name).
				KV("op", "lint").
				KV("ok", err == nil).
				KV("dur", dur).
				Send()

			return ers.Wrapf(err, "lint errors for %q", opts.RootPath)
		}))
	}

	count := 0
	mods := &stw.Map[string, *Module]{}
	reports := &stw.Map[string, testReport]{}

	// Convert module paths to modules, then flatten to packages, then to workers
	for mpath := range iter {
		if count != 0 && !opts.Recursive {
			return fmt.Errorf("module at %q is within %q but recursive mode is not enabled",
				mpath, opts.RootPath)
		}
		count++
		mod, err := Collect(ctx, mpath)
		if err != nil {
			ec.Push(err)
			continue
		}
		mods.Store(mpath, mod)

		// Process each package in the module
		for _, pkg := range mod.Packages {
			pkg := pkg // capture for closure
			ec.Push(main.Push(func(ctx context.Context) error {
				if reports.Check(pkg.PackageName) {
					grip.Errorln("duplicate", pkg.PackageName)
					return nil
				}
				grip.Build().Level(level.Debug).
					Fields(message.Fields{
						"pkg":  pkg.PackageName,
						"op":   "populate",
						"mod":  pkg.ModuleName,
						"path": pkg.LocalDirectory,
					}).
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
				catch.Push(jpm.CreateCommand(ctx).
					ID(fmt.Sprint("test.", pkg.PackageName)).
					Directory(pkg.LocalDirectory).
					AddEnv(global.EnvVarSardisLogQuietSyslog, "true").
					SetOutputSender(level.Info, testOut).
					SetErrorSender(level.Error, testOut).
					PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
					Add(args).
					Run(ctx))
				dur := time.Since(testStart)

				grip.Build().Level(level.Debug).
					Fields(message.Fields{
						"pkg": pkg.PackageName,
						"op":  "test",
						"ok":  catch.Ok(),
						"dur": dur,
					}).
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
							AddEnv(global.EnvVarSardisLogQuietSyslog, "true").
							AppendArgs("go", "tool", "cover", "-html", coverout, "-o", fmt.Sprintf("coverage-%s.html", filepath.Base(result.Info.LocalDirectory))).
							Run(ctx), "coverage html for %s", result.Package))
						ec.Push(jpm.CreateCommand(ctx).
							ID(fmt.Sprint("coverage.report", result.Package)).
							Directory(result.Info.LocalDirectory).
							AddEnv(global.EnvVarSardisLogQuietSyslog, "true").
							SetErrorSender(level.Error, out).
							SetOutputSender(level.Info, sender).
							AppendArgs("go", "tool", "cover", "-func", coverout).
							Run(ctx))
					}
					report(ctx, result.Info, reports.Get(result.Info.PackageName), strings.TrimSpace(buf.String()), time.Since(start), catch.Resolve())
				}

				return catch.Resolve()
			}))
		}
	}

	ec.Push(main.Close())
	ec.Push(pool.Worker().Run(ctx))

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
	pairs := &dt.List[irt.KV[string, any]]{}
	pairs.PushBack(irt.KV[string, any]{Key: "pkg", Value: tr.Package})
	if tr.MissingTests {
		priority = level.Warning
		pairs.PushBack(irt.KV[string, any]{Key: "state", Value: "untested"})
	} else {
		priority = level.Info
		if tr.Cached {
			pairs.PushBack(irt.KV[string, any]{Key: "cached", Value: true})
		}
		if tr.CoverageEnabled {
			if tr.FullyCovered {
				pairs.PushBack(irt.KV[string, any]{Key: "covered", Value: true})
			} else {
				pairs.PushBack(irt.KV[string, any]{Key: "covered", Value: tr.Coverage})
			}
		}
	}
	pairs.PushBack(irt.KV[string, any]{Key: "dur", Value: tr.Duration.Round(time.Millisecond)})

	pairSeq := func(yield func(string, any) bool) {
		elem := pairs.Front()
		for elem != nil {
			kv := elem.Value()
			if !yield(kv.Key, kv.Value) {
				return
			}
			elem = elem.Next()
		}
	}
	out := message.MakeKV(pairSeq)
	out.SetPriority(priority)

	return out
}

func testOutputFilter(
	opts Options,
	mp *stw.Map[string, testReport],
	info PackageInfo,
) send.MessageFormatter {
	return func(m message.Composer) (_ string, err error) {
		defer func() {
			ec := &erc.Collector{}
			ec.Push(err)
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
				report.Duration = erc.Must(time.ParseDuration(parts[2]))
			}
			if opts.ReportCoverage {
				report.Coverage = Percent(erc.Must(strconv.ParseFloat(parts[4][:len(parts[4])-1], 64)))
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
	pwd := erc.Must(os.Getwd())
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

	table := tabby.New()
	replacer := strings.NewReplacer(tr.Package, pfx)
	scanner := bufio.NewScanner(strings.NewReader(coverage))
	for scanner.Scan() {
		in := scanner.Text()
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
	}
	if scanErr := scanner.Err(); scanErr != nil {
		err = erc.Join(err, scanErr)
	}

	msg := grip.Build().KV("pkg", mod.PackageName)

	if tr.MissingTests {
		msg.KV("state", "no tests")
	} else if tr.Cached {
		msg.KV("state", "CACHED")
	}

	if tr.CoverageEnabled {
		if tr.Coverage == 100.0 {
			msg.KV("covered", true)
		} else {
			msg.KV("coverage", tr.Coverage)
			msg.KV("fncount", count)
			msg.KV("covered", numCovered)
		}
		table.Print()
	}

	msg.KV("wal", runtime.Round(time.Millisecond))
	msg.KV("dur", tr.Duration.Round(time.Millisecond))

	switch {
	case err != nil:
		msg.KV("err", err)
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
