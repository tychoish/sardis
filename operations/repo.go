package operations

import (
	"os"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
	"github.com/deciduosity/utility"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Repo() cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "a collections of commands to manage repositories",
		Subcommands: []cli.Command{
			repoUpdate(),
			repoSync(),
			repoCleanup(),
			repoStatus(),
		},
	}
}

func repoUpdate() cli.Command {
	const repoFlagName = "repo"
	return cli.Command{
		Name:  "update",
		Usage: "update a local and remote git repository according to the config",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  repoFlagName,
				Usage: "specify a local repository to updpate",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(), requireStringOrFirstArgSet(repoFlagName)),
		Action: func(c *cli.Context) error {
			repoName := c.String(repoFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			notify := env.Logger()
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
			if err := amboy.ResolveErrors(ctx, queue); err != nil {
				notify.Error(message.Fields{
					"repo": repoName,
					"err":  err.Error(),
				})
				return errors.Wrap(err, "problem found executing jobs")
			}

			notify.Notice(message.Fields{
				"message": "successfully synchronized repository",
				"repo":    repoName,
			})

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func repoCleanup() cli.Command {
	const repoFlagName = "repo"
	return cli.Command{
		Name:  "gc",
		Usage: "run repository cleanup",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  repoFlagName,
				Usage: "specify a local repository to cleanup",
			},
		},
		Before: setAllTailArguements(repoFlagName),
		Action: func(c *cli.Context) error {
			repos := c.StringSlice(repoFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			var allRepos bool
			if len(repos) == 0 {
				allRepos = true
			}

			queue := env.Queue()
			catcher := grip.NewBasicCatcher()
			for _, repo := range env.Configuration().Repo {
				if !allRepos && !utility.StringSliceContains(repos, repo.Name) {
					continue
				}

				catcher.Add(queue.Put(ctx, units.NewRepoCleanupJob(repo.Path)))
			}
			for _, repo := range env.Configuration().Mail {
				if !allRepos && !utility.StringSliceContains(repos, repo.Name) {
					continue
				}

				catcher.Add(queue.Put(ctx, units.NewRepoCleanupJob(repo.Path)))
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func repoSync() cli.Command {
	host, err := os.Hostname()
	grip.Warning(err)
	const nameFlagName = "name"
	return cli.Command{
		Name:  "sync",
		Usage: "sync a local and remote git repository",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: nameFlagName,
			},
			cli.StringFlag{
				Name:  "host",
				Value: host,
			},
		},
		Before: requireStringOrFirstArgSet(nameFlagName),
		Action: func(c *cli.Context) error {
			host := c.String("host")
			name := c.String(nameFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			notify := env.Logger()
			conf := env.Configuration()

			for _, repo := range conf.Mail {
				if name == repo.Name {
					j := units.NewMailSyncJob(repo)
					j.Run(ctx)
					if err := j.Error(); err != nil {
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
				notify.Error(message.Fields{
					"op":  name,
					"err": err.Error(),
				})
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

func repoStatus() cli.Command {
	const repoFlagName = "repo"
	return cli.Command{
		Name:  "status",
		Usage: "report on the status of repos",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  repoFlagName,
				Usage: "specify a local repository to cleanup",
			},
		},
		Before: setAllTailArguements(repoFlagName),
		Action: func(c *cli.Context) error {
			repos := c.StringSlice(repoFlagName)
			var allRepos bool
			if len(repos) == 0 {
				allRepos = true
			}

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			catcher := grip.NewBasicCatcher()
			for _, repo := range env.Configuration().Repo {
				if !allRepos && !utility.StringSliceContains(repos, repo.Name) {
					continue
				}
				j := units.NewRepoStatusJob(repo.Path)
				j.Run(ctx)
				catcher.Add(j.Error())
			}
			for _, repo := range env.Configuration().Mail {
				if !allRepos && !utility.StringSliceContains(repos, repo.Name) {
					continue
				}

				j := units.NewRepoStatusJob(repo.Path)
				j.Run(ctx)
				catcher.Add(j.Error())
			}
			return catcher.Resolve()
		},
	}

}
