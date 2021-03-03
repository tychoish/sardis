package operations

import (
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Admin() cli.Command {
	return cli.Command{
		Name: "admin",
		Subcommands: []cli.Command{
			configCheck(),
			nightly(),
			setupLinks(),
		},
	}
}

func configCheck() cli.Command {
	return cli.Command{
		Name:   "config",
		Usage:  "validated configuration",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			conf := sardis.GetEnvironment().Configuration()
			err := conf.Validate()
			if err == nil {
				grip.Info("configuration is valid")
			}
			return errors.Wrap(err, "configuration validation error")
		},
	}
}

func nightly() cli.Command {
	return cli.Command{
		Name:   "nightly",
		Usage:  "run nightly config operation",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()
			queue := env.Queue()
			catcher := grip.NewBasicCatcher()
			notify := env.Logger()
			ctx, cancel := env.Context()
			defer cancel()
			startAt := time.Now()

			for idx := range conf.Links {
				catcher.Add(queue.Put(ctx, units.NewSymlinkCreateJob(conf.Links[idx])))
			}

			for idx := range conf.Repo {
				catcher.Add(queue.Put(ctx, units.NewRepoCleanupJob(conf.Repo[idx].Path)))
			}

			for idx := range conf.System.Services {
				grip.Info(conf.System.Services[idx])
				catcher.Add(queue.Put(ctx, units.NewSystemServiceSetupJob(conf.System.Services[idx])))
			}

			stat := queue.Stats(ctx)
			grip.Debug(stat)

			if stat.Total > 0 {
				amboy.WaitInterval(ctx, queue, 20*time.Millisecond)
			}
			catcher.Add(amboy.ResolveErrors(ctx, queue))

			notify.WarningWhen(catcher.HasErrors() || time.Since(startAt) > time.Hour,
				message.Fields{
					"errs":    catcher.Len(),
					"jobs":    stat.Total,
					"msg":     "problem running nightly tasks",
					"dur_sec": time.Since(startAt).Seconds(),
				})

			return catcher.Resolve()
		},
	}
}
