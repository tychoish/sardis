package operations

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strings"
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
			syncAllRepos(),
			updateRepos(),
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
			ctx, cancel := context.WithCancel(env.Context())
			defer cancel()

			job := units.NewRepoSyncJob(c.String("host"), c.String("path"), nil, nil)
			grip.Infof("starting: %s", job.ID())
			job.Run(ctx)

			return errors.Wrap(job.Error(), "job encountered problem")
		},
	}
}

func syncAllRepos() cli.Command {
	return cli.Command{
		Name:   "sync",
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

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			amboy.WaitCtxInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func updateRepos() cli.Command {
	hostname, err := os.Hostname()
	grip.Warning(err)

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

			for _, repo := range conf.Repo {
				if repo.LocalSync {
					catcher.Add(queue.Put(units.NewLocalRepoSyncJob(repo.Path)))
				}

				for _, mirror := range repo.Mirrors {
					if strings.Contains(mirror, hostname) {
						grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
							repo.Path, mirror, hostname)
						continue
					}
					catcher.Add(queue.Put(units.NewRepoSyncRemoteJob(mirror, repo.Path, repo.Pre, repo.Post)))
				}
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			amboy.WaitCtxInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}

}
