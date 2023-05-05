package operations

import (
	"context"
	"errors"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
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
	return cmdr.MakeCommander().
		SetName("fetch").
		SetUsage("donwload source to build directory").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify the name of a package").
			Flag()).
		With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) ([]string, error) {
			packages := append(cc.StringSlice(nameFlagName), cc.Args()...)
			if len(packages) == 0 {
				return nil, errors.New("must specify one package to fetch")
			}

			return packages, nil
		}).SetAction(func(ctx context.Context, packages []string) error {
			queue, run := units.SetupWorkers()

			for _, name := range packages {
				queue.PushBack(units.NewArchFetchAurJob(name, true))
			}

			return run(ctx)
		}).Add)
}

func buildPkg() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("build").
		SetUsage("download and build package").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify the name of a package").
			Flag()).
		With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) ([]string, error) {
			packages := append(cc.StringSlice(nameFlagName), cc.Args()...)
			if len(packages) == 0 {
				return nil, errors.New("must specify one package to fetch")
			}

			return packages, nil
		}).SetAction(func(ctx context.Context, packages []string) error {
			queue, run := units.SetupWorkers()

			for _, name := range packages {
				queue.PushBack(units.NewArchAbsBuildJob(name))
			}

			return run(ctx)
		}).Add)
}

func installAur() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("install").
		SetUsage("combination build download and install").
		Flags(cmdr.FlagBuilder([]string{}).
			SetName(nameFlagName, "n").
			SetUsage("specify the name of a package").
			Flag()).
		With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) ([]string, error) {
			packages := append(cc.StringSlice(nameFlagName), cc.Args()...)
			if len(packages) == 0 {
				return nil, errors.New("must specify one package to fetch")
			}
			return packages, nil
		}).SetAction(func(ctx context.Context, packages []string) error {
			catcher := &erc.Collector{}

			for _, name := range packages {
				job := units.NewArchFetchAurJob(name, true)
				if err := job(ctx); err != nil {
					catcher.Add(err)
					continue
				}

				catcher.Add(units.NewArchAbsBuildJob(name)(ctx))
			}

			return catcher.Resolve()
		}).Add)
}

func setupArchLinux() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("setup").
		SetUsage("bootstrap/setup system according to packages described").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			catcher := &erc.Collector{}

			pkgs := []string{}
			for _, pkg := range conf.System.Arch.Packages {
				pkgs = append(pkgs, pkg.Name)
			}

			grip.Info(message.Fields{
				"path":     conf.System.Arch.BuildPath,
				"packages": len(pkgs),
				"aur":      len(conf.System.Arch.AurPackages),
			})

			catcher.Add(units.NewArchInstallPackageJob(pkgs)(ctx))

			for _, pkg := range conf.System.Arch.AurPackages {
				grip.Info(message.Fields{
					"name":   pkg.Name,
					"update": pkg.Update,
				})

				if err := units.NewArchFetchAurJob(pkg.Name, pkg.Update)(ctx); err != nil {
					catcher.Add(err)
					continue
				}

				catcher.Add(units.NewArchAbsBuildJob(pkg.Name)(ctx))
			}

			return catcher.Resolve()
		}).Add)
}
