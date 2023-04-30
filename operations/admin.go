package operations

import (
	"context"

	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Admin(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "admin",
		Usage: "local sysadmin scripts",
		Subcommands: []cli.Command{
			configCheck(ctx),
			nightly(ctx),
			setupLinks(ctx),
		},
	}
}

func configCheck(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "config",
		Usage: "validated configuration",
		Action: func(c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
			err := conf.Validate()
			if err == nil {
				grip.Info("configuration is valid")
			}
			return err
		},
	}
}

func nightly(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "nightly",
		Usage: "run nightly config operation",
		Action: func(c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			jobs, run := units.SetupQueue(amboy.RunJob)

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
