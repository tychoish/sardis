package operations

import (
	"context"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

const nameFlagName = "name"

func ArchLinux(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "arch",
		Usage: "arch linux management options",
		Subcommands: []cli.Command{
			fetchAur(ctx),
			buildPkg(ctx),
			installAur(ctx),
			setupArchLinux(ctx),
		},
	}
}

func fetchAur(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "fetch",
		Usage: "donwload source to build directory",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: addRemanderToStringSliceFlag(nameFlagName),
		Action: func(c *cli.Context) error {
			queue, run := units.SetupQueue(amboy.RunJob)

			for _, name := range c.StringSlice(nameFlagName) {
				queue.PushBack(units.NewArchFetchAurJob(name, true))
			}

			return run(ctx)
		},
	}
}

func buildPkg(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "build",
		Usage: "donwload source to build directory",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: addRemanderToStringSliceFlag(nameFlagName),
		Action: func(c *cli.Context) error {
			catcher := &erc.Collector{}

			for _, name := range c.StringSlice(nameFlagName) {
				catcher.Add(amboy.RunJob(ctx, units.NewArchAbsBuildJob(name)))
			}

			return catcher.Resolve()
		},
	}
}

func installAur(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "install",
		Usage: "combination download+build+install",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: addRemanderToStringSliceFlag(nameFlagName),
		Action: func(c *cli.Context) error {
			catcher := &erc.Collector{}

			for _, name := range c.StringSlice(nameFlagName) {
				if err := amboy.RunJob(ctx, units.NewArchFetchAurJob(name, true)); err != nil {
					catcher.Add(err)
					continue
				}

				catcher.Add(amboy.RunJob(ctx, units.NewArchAbsBuildJob(name)))
			}

			return catcher.Resolve()
		},
	}
}

func setupArchLinux(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "setup",
		Usage:  "bootstrap/setup system according to packages described",
		Flags:  []cli.Flag{},
		Before: addRemanderToStringSliceFlag(nameFlagName),
		Action: func(c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
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

			catcher.Add(amboy.RunJob(ctx, units.NewArchInstallPackageJob(pkgs)))

			for _, pkg := range conf.System.Arch.AurPackages {
				grip.Info(message.Fields{
					"name":   pkg.Name,
					"update": pkg.Update,
				})

				if err := amboy.RunJob(ctx, units.NewArchFetchAurJob(pkg.Name, pkg.Update)); err != nil {
					catcher.Add(err)
					continue
				}

				catcher.Add(amboy.RunJob(ctx, units.NewArchAbsBuildJob(pkg.Name)))
			}

			return catcher.Resolve()
		},
	}
}
