package operations

import (
	"path/filepath"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli"
)

func Repo() cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "a collections of commands to manage repositories",
		Subcommands: []cli.Command{
			updateRepos(),
			syncRepo(),
		},
	}
}

func updateRepos() cli.Command {
	return cli.Command{
		Name:   "update",
		Usage:  "update a local and remote git repository according to the config",
		Before: requireConfig(),
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "repo",
				Usage: "specify a local repository to updpate",
			},
		},
		Action: func(c *cli.Context) error {
			repoName := c.String("repo")

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			defer env.Close(ctx)

			queue := env.Queue()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()

			catcher.Add(units.SyncRepo(ctx, queue, conf, repoName))

			for _, link := range conf.Links {
				catcher.Add(queue.Put(ctx, units.NewSymlinkCreateJob(link)))
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func syncRepo() cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "sync a local and remote git repository",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Value: filepath.Join(util.GetHomeDir(), "mail"),
			},
			cli.StringFlag{
				Name:  "host",
				Value: "LOCAL",
			},
		},
		Action: func(c *cli.Context) error {
			host := c.String("host")
			name := c.String("name")

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			defer env.Close(ctx)

			notify := env.Logger()
			conf := env.Configuration()

			for _, repo := range conf.Mail {
				if name == repo.Name {
					j := units.NewMailSyncJob(repo)
					j.Run(ctx)
					if err := j.Error(); err != nil {
						notify.Error(message.WrapError(err, message.Fields{
							"message": "encountered problem syncing repository",
							"host":    host,
							"repo":    name,
						}))

						return errors.Wrap(j.Error(), "problem syncing mail repo")
					}
					return nil
				}
			}

			queue := env.Queue()
			if err := units.SyncRepo(ctx, queue, conf, name); err != nil {
				return errors.Wrap(err, "problem queuing jobs")
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)

			if err := amboy.ResolveErrors(ctx, queue); err != nil {
				notify.Error(message.WrapError(err, message.Fields{
					"message": "encountered problem syncing repository",
					"host":    host,
					"repo":    name,
				}))

				return errors.Wrap(err, "problem found executing jobs")
			}

			notify.Notice(message.Fields{
				"message": "successfully synchronized repository",
				"host":    host,
				"repo":    name,
			})

			return nil
		},
	}
}
