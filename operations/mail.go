package operations

import (
	"os/user"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
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
			syncMailRepo(),
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
			defer cancel()
			defer env.Close(ctx)

			job := units.NewMailUpdaterJob(c.String("mail"), c.String("mu"), c.String("daemon"), c.Bool("rebuild"))
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}
}

func syncMailRepo() cli.Command {
	const nameFlagName = "name"
	return cli.Command{
		Name:  "repo",
		Usage: "sync a local and remote git repository",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  nameFlagName,
				Value: "personal",
				Usage: "specify the name of the repository",
			},
			cli.BoolFlag{
				Name:  "skipUpdate",
				Usage: "when specified, do not update the mail directory",
			},
		},
		Before: requireStringOrFirstArgSet(nameFlagName),
		Action: func(c *cli.Context) error {
			name := c.String(nameFlagName)
			skipUpdate := c.Bool("skipUpdate")

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			defer env.Close(ctx)
			var conf sardis.MailConf
			grip.Info(name)
			for _, repo := range env.Configuration().Mail {
				grip.Notice(repo)
				if repo.Name == name {
					conf = repo
					break
				}
			}

			if conf.Name == "" {
				return errors.Errorf("no matching for '%s' defined", name)
			}

			notify := env.Logger()

			job := units.NewMailSyncJob(conf)
			grip.Infof("starting: %s", job.ID())
			job.Run(ctx)

			err := job.Error()
			if err != nil {
				notify.Error(message.WrapError(err, message.Fields{
					"message": "encountered problem syncing repository",
					"name":    conf.Name,
					"path":    conf.Path,
				}))
				return err
			}

			if !skipUpdate {
				job = units.NewMailUpdaterJob(conf.Path, conf.MuPath, conf.Emacs, false)
				grip.Infof("updating mu db: %s", job.ID())
				job.Run(ctx)
				if err = job.Error(); err != nil {
					notify.Error(message.WrapError(err, message.Fields{
						"message": "problem updating mail database",
						"emacs":   conf.Emacs,
						"name":    conf.Name,
						"path":    conf.Path,
					}))
					return err
				}
			}

			notify.Notice(message.Fields{
				"message": "successfully synchronized repository",
				"name":    conf.Name,
				"path":    conf.Path,
			})

			return nil
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
			defer cancel()
			defer env.Close(ctx)

			notify := env.Logger()
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
			err := errors.WithStack(amboy.ResolveErrors(ctx, queue))
			if err != nil {
				notify.Error(message.WrapError(err, "completed sync mail operation with error"))
				return err
			}

			notify.Notice("completed mail sync operation successfully")
			return nil
		},
	}
}
