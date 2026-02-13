package operations

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

const nameFlagName = "name"

func ArchLinux() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("arch").
		SetUsage("arch linux management options").
		Subcommanders(
			fetchAur(),
			buildPkg(),
			installAur(),
			dumpArchPackages(),
			setupArchLinux(),
		)
}

func fetchAur() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("fetch").
		SetUsage("download source to build directory"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch
			workers := func(yield func(fnx.Worker) bool) {
				for _, name := range args.arg {
					if !yield(conf.FetchPackageFromAUR(name, true)) {
						return
					}
				}
			}
			return wpa.RunWithPool(workers,
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			)(ctx)
		})
}

func buildPkg() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("build").
		SetUsage("build a package"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch
			workers := func(yield func(fnx.Worker) bool) {
				for _, name := range args.arg {
					if !yield(conf.BuildPackageInABS(name)) {
						return
					}
				}
			}
			return wpa.RunWithPool(workers,
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			)(ctx)
		})
}

func installAur() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("install").
		SetUsage("fetch AUR package to the ABS directory, and install it"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch
			workers := func(yield func(fnx.Worker) bool) {
				for _, name := range args.arg {
					if !yield(conf.FetchPackageFromAUR(name, true).Join(conf.BuildPackageInABS(name))) {
						return
					}
				}
			}
			return wpa.RunWithPool(workers,
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfWorkerPerCPU(),
			)(ctx)
		})
}

func dumpArchPackages() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("export").
		SetUsage("dump installed package database"),
		nameFlagName, func(ctx context.Context, args *withConf[string]) (err error) {
			path := args.arg
			conf := args.conf.System.Arch
			var output io.WriteCloser
			switch {
			case path == "":
				output = os.Stdout
			case util.FileExists(path):
				return fmt.Errorf("file %s alredy exists")
			case !util.FileExists(filepath.Dir(path)):
				return fmt.Errorf("cannot write file %q, create parent directory", path)
			case HasSuffix(irt.Args("json", "yaml", "bson", "yml", "jsonl"), path):
				return fmt.Errorf("unknown serialization format for %s", filepath.Ext(path))
			default:
				output, err := os.Create(path)
				if err != nil {
					return err
				}

				defer func() { err = erc.Join(err, output.Close()) }()
			}

			if err := conf.Discovery().Run(ctx); err != nil {
				return err
			}

			data := conf.Export()
			data.DEP = nil

			defer grip.Info(message.NewKV().
				KV("ABS", len(data.ABS)).
				KV("AUR", len(data.AUR)).
				KV("USR", len(data.USR)),
			)
			return util.MarshalerForFile(path).Write(conf.Export()).Into(output)
		})
}

func setupArchLinux() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("setup").
		SetUsage("bootstrap/setup system according to packages described").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			ec := &erc.Collector{}

			arch := &conf.System.Arch

			grip.Info(message.Fields{
				"path":     arch.BuildPath,
				"packages": len(arch.Packages),
			})

			wpq := dt.List[fnx.Worker]{}

			wpq.PushBack(arch.InstallPackages())

			for _, pkg := range arch.Packages {
				if pkg.State.InDistRepos {
					continue
				}
				grip.Info(message.Fields{
					"name":   pkg.Name,
					"update": pkg.ShouldUpdate,
				})

				wpq.PushBack(arch.FetchPackageFromAUR(pkg.Name, pkg.ShouldUpdate).
					Join(arch.BuildPackageInABS(pkg.Name)))

			}

			workers := func(yield func(fnx.Worker) bool) {
				for elem := wpq.Front(); elem != nil; elem = elem.Next() {
					if !yield(elem.Value()) {
						return
					}
				}
			}
			ec.Push(wpa.RunWithPool(workers)(ctx))

			return ec.Resolve()
		}).Add)
}
