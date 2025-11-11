package operations

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
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
			setupArchLinux(),
		)
}

func fetchAur() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("fetch").
		SetUsage("download source to build directory"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch
			return fun.Convert(fnx.MakeConverter(func(name string) fnx.Worker {
				return conf.FetchPackageFromAUR(name, true)
			})).Stream(fun.SliceStream(args.arg)).Parallel(
				func(ctx context.Context, op fnx.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}

func buildPkg() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("build").
		SetUsage("build a package"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch

			return fun.Convert(fnx.MakeConverter(func(name string) fnx.Worker {
				return conf.BuildPackageInABS(name)
			})).Stream(fun.SliceStream(args.arg)).Parallel(
				func(ctx context.Context, op fnx.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}

func installAur() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("install").
		SetUsage("fetch AUR package to the ABS directory, and install it"),
		nameFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf.System.Arch

			return fun.Convert(fnx.MakeConverter(func(name string) fnx.Worker {
				return conf.FetchPackageFromAUR(name, true).Join(conf.BuildPackageInABS(name))
			})).Stream(fun.SliceStream(args.arg)).Parallel(
				func(ctx context.Context, op fnx.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
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

			fun.MAKE.WorkerPool(wpq.StreamFront()).Operation(ec.Push).Run(ctx)

			return ec.Resolve()
		}).Add)
}
