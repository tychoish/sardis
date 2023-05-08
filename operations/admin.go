package operations

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli/v2"
)

func ResolveConfiguration(ctx context.Context, cc *cli.Context) (*sardis.Configuration, error) {
	conf, err := sardis.LoadConfiguration(cc.String("conf"))

	if err != nil {
		return nil, err
	}

	conf.Settings.Logging.EnableJSONFormating = cc.Bool("jsonLog")
	conf.Settings.Logging.DisableStandardOutput = cc.Bool("quietStdOut")
	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))

	return conf, nil
}

func Admin() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("admin").
		SetUsage("local systems administration scripts").
		Subcommanders(
			configCheck(),
			nightly(),
			setupLinks(),
		)
}

func configCheck() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("config").
		SetUsage("validated configuration").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			// this is redundant, as the resolve
			// configuration does this correctly.
			err := conf.Validate()
			if err == nil {
				grip.Info("configuration is valid")
			}
			return err
		}).Add)
}

func nightly() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("nightly").
		SetUsage("run nightly config operation").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
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
		}).Add)
}
