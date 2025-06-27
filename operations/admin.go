package operations

import (
	"context"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
)

func ResolveConfiguration(ctx context.Context, cc *cli.Context) (*sardis.Configuration, error) {
	if sardis.HasAppConfiguration(ctx) {
		return sardis.AppConfiguration(ctx), nil
	}

	conf, err := sardis.LoadConfiguration(cc.String("conf"))
	if err != nil {
		return nil, err
	}

	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))

	conf.Settings.Logging.DisableSyslog = cc.Bool("quietSyslog") || os.Getenv("SARDIS_LOG_QUIET_SYSLOG") != ""
	conf.Settings.Logging.DisableStandardOutput = cc.Bool("quietStdOut") || os.Getenv("SARDIS_LOG_QUIET_STDOUT") != ""
	conf.Settings.Logging.EnableJSONFormating = cc.Bool("jsonLog") || os.Getenv("SARDIS_LOG_FORMAT_JSON") != ""
	conf.Settings.Logging.EnableJSONColorFormatting = cc.Bool("colorJsonLog") || os.Getenv("SARDIS_LOG_COLOR_JSON") != ""

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
			ec := &erc.Collector{}
			jobs, run := units.SetupWorkers(ec)

			for idx := range conf.Links {
				jobs.PushBack(units.NewSymlinkCreateJob(conf.Links[idx]))
			}

			for idx := range conf.Repo {
				jobs.PushBack(units.NewRepoCleanupJob(conf.Repo[idx].Path))
			}

			for idx := range conf.System.Services {
				jobs.PushBack(units.NewSystemServiceSetupJob(conf.System.Services[idx]))
			}
			ec.Add(run(ctx))
			return ec.Resolve()
		}).Add)
}
