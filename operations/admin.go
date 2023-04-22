package operations

import (
	"context"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Admin(ctx context.Context) cli.Command {
	return cli.Command{
		Name: "admin",
		Subcommands: []cli.Command{
			configCheck(ctx),
			nightly(ctx),
			setupLinks(ctx),
		},
	}
}

func configCheck(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "config",
		Usage:  "validated configuration",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			conf := sardis.GetEnvironment(ctx).Configuration()
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
		Name:   "nightly",
		Usage:  "run nightly config operation",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)
			conf := env.Configuration()
			queue := env.Queue()
			catcher := &erc.Collector{}
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
					"jobs":    stat.Total,
					"msg":     "problem running nightly tasks",
					"dur_sec": time.Since(startAt).Seconds(),
				})

			return catcher.Resolve()
		},
	}
}
