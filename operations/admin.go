package operations

import (
	"context"

	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Admin() cli.Command {
	return cli.Command{
		Name:  "admin",
		Usage: "local sysadmin scripts",
		Subcommands: []cli.Command{
			configCheck(),
			nightly(),
			setupLinks(),
		},
	}
}

func configCheck() cli.Command {
	return cli.Command{
		Name:  "config",
		Usage: "validated configuration",
		Action: func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
			err := conf.Validate()
			if err == nil {
				grip.Info("configuration is valid")
			}
			return err
		},
	}
}

func nightly() cli.Command {
	return cli.Command{
		Name:  "nightly",
		Usage: "run nightly config operation",
		Action: func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			jobs, run := units.SetupWorkers()

			for idx := range conf.Links {
				jobs.PushBack(units.NewSymlinkCreateJob(conf.Links[idx]))
			}

			for idx := range conf.Repo {
				jobs.PushBack(units.NewRepoCleanupJob(conf.Repo[idx].Path))
			}

			for idx := range conf.System.Services {
				jobs.PushBack(units.NewSystemServiceSetupJob(conf.System.Services[idx]))
			}

			return run(ctx)
		},
	}
}
