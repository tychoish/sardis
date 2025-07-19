package operations

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
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
		SetUsage("download source to build directory").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify a package or packages to download from the AUR").
			Flag()),
		"name", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			conf := args.conf.System.Arch
			return fun.MakeConverter(func(name string) fun.Worker {
				return conf.FetchPackageFromAUR(name, true)
			}).Stream(fun.SliceStream(args.ops)).Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}

func buildPkg() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("build").
		SetUsage("build a package").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify a package or packages from the AUR").
			Flag()),
		"name", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			conf := args.conf.System.Arch

			return fun.MakeConverter(func(name string) fun.Worker {
				return conf.BuildPackageInABS(name)
			}).Stream(fun.SliceStream(args.ops)).Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}

func installAur() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("install").
		SetUsage("fetch AUR package to the ABS directory, and install it").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify a package or packages from the AUR").
			Flag()),
		"name", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			conf := args.conf.System.Arch

			return fun.MakeConverter(func(name string) fun.Worker {
				return conf.FetchPackageFromAUR(name, true).Join(conf.BuildPackageInABS(name))
			}).Stream(fun.SliceStream(args.ops)).Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
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
				"aur":      len(arch.AurPackages),
			})

			arch.InstallPackages().Operation(ec.Push).Run(ctx)

			for _, pkg := range arch.AurPackages {
				grip.Info(message.Fields{
					"name":   pkg.Name,
					"update": pkg.Update,
				})

				arch.FetchPackageFromAUR(pkg.Name, pkg.Update).
					Join(arch.BuildPackageInABS(pkg.Name)).
					Operation(ec.Push).
					Run(ctx)
			}

			return ec.Resolve()
		}).Add)
}
