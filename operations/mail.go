package operations

import (
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
			syncAllMailRepos(),
		},
	}
}

func updateDB() cli.Command {
	usr, err := user.Current()
	grip.Warning(err)

	if usr == nil {
		usr = &user.User{}
	}

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
				Value: usr.Username,
			},
			cli.BoolFlag{
				Name:  "rebuild",
				Usage: "should perform a full rebuild of the index",
			},
		},
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer env.Close(ctx)
			defer cancel()

			job := units.NewMailUpdaterJob(c.String("mail"), c.String("mu"), c.String("daemon"), c.Bool("rebuild"))
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}
}

func syncRepo() cli.Command {
	return cli.Command{
		Name:  "repo",
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
			ctx, cancel := env.Context()
			defer env.Close(ctx)
			defer cancel()

			job := units.NewRepoSyncJob(c.String("host"), c.String("path"), nil, nil)
			grip.Infof("starting: %s", job.ID())
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}
}

func syncAllMailRepos() cli.Command {
	return cli.Command{
		Name:   "sync",
		Usage:  "sync all mail repos from the config",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer env.Close(ctx)
			defer cancel()

			queue := env.Queue()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()

			for _, mdir := range conf.Mail {
				catcher.Add(queue.Put(ctx, units.NewMailSyncJob(mdir)))
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)
			return errors.WithStack(amboy.ResolveErrors(ctx, queue))
		},
	}
}
