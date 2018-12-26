package operations

import (
	"context"
	"os/user"
	"path/filepath"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli"
)

func Mail() cli.Command {
	return cli.Command{
		Name:  "mail",
		Usage: "a collections of commands to manage the maildir deployment",
		Subcommands: []cli.Command{
			updateDB(),
			syncRepo(),
			updateMail(),
		},
	}
}

func updateDB() cli.Command {
	user, err := user.Current()
	grip.Warning(err)

	return cli.Command{
		Name: "mu",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "mail",
				Usage: "specify the path to the Maildir",
				Value: "~/mail",
			},
			cli.StringFlag{
				Name:  "mu",
				Usage: "specify the path to the muhome",
				Value: "~/.mu",
			},
			cli.StringFlag{
				Name:  "daemon",
				Usage: "name of emacs deamon",
				Value: user.Username,
			},
			cli.BoolFlag{
				Name:  "rebuild",
				Usage: "should perform a full rebuild of the index",
			},
		},
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := context.WithCancel(env.Context())
			defer cancel()

			job := units.NewMailUpdaterJob(c.String("mail"), c.String("mu"), c.String("daemon"), c.Bool("rebuild"))
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}
}

func syncRepo() cli.Command {
	return cli.Command{
		Name:  "sync",
		Usage: "sync a local and remote git repository",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "path",
				Value: filepath.Join(util.GetHomeDir(), "mail"),
			},
			cli.StringFlag{
				Name:  "host",
				Value: "LOCAL",
			},
		},
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := context.WithCancel(env.Context())
			defer cancel()

			job := units.NewRepoSyncJob(c.String("host"), c.String("path"))
			grip.Infof("starting: %s", job.ID())
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}

}

func updateMail() cli.Command {
	return cli.Command{
		Name:   "update",
		Usage:  "update a local and remote git repository",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := context.WithCancel(env.Context())
			defer cancel()

			queue := env.Queue()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()

			for _, mdir := range conf.Mail {
				catcher.Add(queue.Put(units.NewMailSyncJob(mdir)))
			}

			for _, repo := range conf.Repo {
				if !repo.ShouldSync {
					continue
				}
				catcher.Add(queue.Put(units.NewLocalRepoSyncJob(repo.Path)))
			}
			grip.EmergencyFatal(catcher.Resolve())
			amboy.WaitCtxInterval(ctx, queue, time.Millisecond)
			return nil
		},
	}
}
