package operations

import (
	"github.com/mongodb/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

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
			cli.StringFlag{
				Name:  "name, n",
				Usage: "specify the name of a package",
			},
		},
		// TODO: before to possitional, plus require config
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx := env.Context()

			job := units.NewArchFetchAurJob(c.String("name"), true)
			job.Run(ctx)
			return job.Error()
		},
	}
}

func buildPkg() cli.Command {
	return cli.Command{
		Name:  "build",
		Usage: "donwload source to build directory",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name, n",
				Usage: "specify the name of a package",
			},
		},
		// TODO: before to possitional, plus require config
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			pkg := c.String("name")
			ctx := env.Context()

			job := units.NewArchAbsBuildJob(pkg)
			job.Run(ctx)

			return job.Error()
		},
	}
}

func installAur() cli.Command {
	return cli.Command{
		Name:  "install",
		Usage: "combination download+build+install",
		Flags: []cli.Flag{},
		// TODO: before to possitional, plus require config
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()
			grip.Info("would buil package")

			grip.Infoln(conf != nil)

			return nil
		},
	}
}

func setupArchLinux() cli.Command {
	return cli.Command{
		Name:  "setup",
		Usage: "bootstrap/setup system according to packages described",
		Flags: []cli.Flag{},
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()
			ctx := env.Context()

			pkgs := []string{}
			for _, pkg := range conf.Arch.Packages {
				pkgs = append(pkgs, pkg.Name)
			}

			job := units.NewArchInstallPackageJob(pkgs)
			job.Run(ctx)
			catcher.Add(job.Error())

			for _, pkg := range conf.Arch.AurPackages {
				job := units.NewArchFetchAurJob(pkg.Name, pkg.Update)
				job.Run(ctx)
				err := job.Error()

				if err != nil {
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
