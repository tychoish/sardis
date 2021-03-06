package operations

import (
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

const nameFlagName = "name"

func ArchLinux() cli.Command {
	return cli.Command{
		Name:  "arch",
		Usage: "arch linux management options",
		Subcommands: []cli.Command{
			fetchAur(),
			buildPkg(),
			installAur(),
			setupArchLinux(),
		},
	}
}

func fetchAur() cli.Command {
	return cli.Command{
		Name:  "fetch",
		Usage: "donwload source to build directory",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: mergeBeforeFuncs(addRemanderToStringSliceFlag(nameFlagName), requireConfig()),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			catcher := grip.NewBasicCatcher()

			for _, name := range c.StringSlice(nameFlagName) {
				job := units.NewArchFetchAurJob(name, true)
				job.Run(ctx)
				catcher.Add(job.Error())
			}

			return catcher.Resolve()
		},
	}
}

func buildPkg() cli.Command {
	return cli.Command{
		Name:  "build",
		Usage: "donwload source to build directory",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: mergeBeforeFuncs(addRemanderToStringSliceFlag(nameFlagName), requireConfig()),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			catcher := grip.NewBasicCatcher()

			for _, name := range c.StringSlice(nameFlagName) {
				job := units.NewArchAbsBuildJob(name)
				job.Run(ctx)
				catcher.Add(job.Error())
			}

			return catcher.Resolve()
		},
	}
}

func installAur() cli.Command {
	return cli.Command{
		Name:  "install",
		Usage: "combination download+build+install",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(nameFlagName, "n"),
				Usage: "specify the name of a package",
			},
		},
		Before: mergeBeforeFuncs(addRemanderToStringSliceFlag(nameFlagName), requireConfig()),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			catcher := grip.NewBasicCatcher()

			for _, name := range c.StringSlice(nameFlagName) {
				job := units.NewArchFetchAurJob(name, true)
				job.Run(ctx)

				if err := job.Error(); err != nil {
					catcher.Add(err)
					continue
				}

				job = units.NewArchAbsBuildJob(name)
				job.Run(ctx)
				catcher.Add(job.Error())
			}

			return catcher.Resolve()
		},
	}
}

func setupArchLinux() cli.Command {
	return cli.Command{
		Name:   "setup",
		Usage:  "bootstrap/setup system according to packages described",
		Flags:  []cli.Flag{},
		Before: mergeBeforeFuncs(addRemanderToStringSliceFlag(nameFlagName), requireConfig()),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()
			ctx, cancel := env.Context()
			defer cancel()

			pkgs := []string{}
			for _, pkg := range conf.System.Arch.Packages {
				pkgs = append(pkgs, pkg.Name)
			}

			grip.Info(message.Fields{
				"path":     conf.System.Arch.BuildPath,
				"packages": len(pkgs),
				"aur":      len(conf.System.Arch.AurPackages),
			})
			job := units.NewArchInstallPackageJob(pkgs)
			job.Run(ctx)
			catcher.Add(job.Error())

			for _, pkg := range conf.System.Arch.AurPackages {
				grip.Info(message.Fields{
					"name":   pkg.Name,
					"update": pkg.Update,
				})

				job := units.NewArchFetchAurJob(pkg.Name, pkg.Update)
				job.Run(ctx)

				if err := job.Error(); err != nil {
					catcher.Add(err)
					continue
				}

				job = units.NewArchAbsBuildJob(pkg.Name)
				job.Run(ctx)
				catcher.Add(job.Error())
			}

			return catcher.Resolve()
		},
	}
}
