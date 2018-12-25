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
			ctx, cancel := context.WithCancel(context.Background())
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
				Value: filepath.Join(getHomeDir(), "mail"),
			},
			cli.StringFlag{
				Name:  "host",
				Value: "LOCAL",
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
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
		Name:  "update",
		Usage: "update a local and remote git repository",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "conf, c",
				Value: filepath.Join(getHomeDir(), ".sardis.yaml"),
			}},
		Action: func(c *cli.Context) error {
			path := c.String("conf")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			grip.EmergencyFatal(configureQueue())
			queue, err := sardis.GetQueue()
			grip.EmergencyFatal(err)
			grip.EmergencyFatal(queue.Start(ctx))
			conf, err := sardis.LoadConfiguration(path)
			grip.EmergencyFatal(errors.Wrapf(err, "problem loading config from '%s'", path))

			catcher := grip.NewBasicCatcher()
			for _, mdir := range conf.Mail {
				catcher.Add(queue.Put(units.NewMailSyncJob(mdir)))
			}

			grip.EmergencyFatal(catcher.Resolve())
			amboy.WaitCtxInterval(ctx, queue, 100*time.Microsecond)

			return nil
		},
	}
}
