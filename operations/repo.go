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

func Repo() cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "a collections of commands to manage repositories",
		Subcommands: []cli.Command{
			repoClone(),
			repoUpdate(),
			repoCleanup(),
			repoStatus(),
			repoFetch(),
		},
	}
}

func repoUpdate() cli.Command {
	const repoTagFlagName = "repo"
	return cli.Command{
		Name:  "update",
		Usage: "update a local and remote git repository according to the config",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  repoTagFlagName,
				Usage: "specify tag of repos to update",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(), requireStringOrFirstArgSet(repoTagFlagName)),
		Action: func(c *cli.Context) error {
			tagName := c.String(repoTagFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()

			repos := conf.GetTaggedRepos(tagName)
			if len(repos) == 0 {
				return errors.Errorf("no tagged repository named '%s' configured", tagName)
			}

			queue := env.Queue()
			catcher := grip.NewBasicCatcher()

			hadChanges := []string{}

			for idx := range repos {
				catcher.Add(units.SyncRepo(ctx, queue, &repos[idx]))
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			started := time.Now()
			stat := queue.Stats(ctx)
			grip.Info(message.Fields{
				"op":      "repo sync",
				"message": "waiting for jobs to complete",
				"jobs":    stat.Total,
				"tag":     tagName,
			})
			amboy.WaitInterval(ctx, queue, 10*time.Millisecond)
			catcher.Wrap(amboy.ResolveErrors(ctx, queue), "jobs encountered error")

			// QUESTION: should we send notification here
			grip.Notice(message.Fields{
				"op":       "repo sync",
				"code":     "success",
				"repo":     tagName,
				"changed":  hadChanges,
				"dur_sec":  time.Since(started).Seconds(),
				"err":      catcher.HasErrors(),
				"num_errs": catcher.Len(),
				"jobs":     stat.Total,
			})

			return catcher.Resolve()
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
			tags := c.StringSlice(repoFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			var repos []sardis.RepoConf
			if len(tags) == 0 {
				// all repos
				repos = env.Configuration().Repo
			} else {
				for _, tag := range tags {
					repos = append(repos, env.Configuration().GetTaggedRepos(tag)...)
				}
			}

			queue := env.Queue()
			catcher := grip.NewBasicCatcher()
			for _, repo := range repos {
				catcher.Add(queue.Put(ctx, units.NewRepoCleanupJob(repo.Path)))
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func repoClone() cli.Command {
	const nameFlagName = "name"
	return cli.Command{
		Name:  "clone",
		Usage: "clone a repository or all matching repositories",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name: nameFlagName,
			},
		},
		Before: requireStringOrFirstArgSet(nameFlagName),
		Action: func(c *cli.Context) error {
			name := c.String(nameFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()

			catcher := grip.NewBasicCatcher()
			queue := env.Queue()

			repos := conf.GetTaggedRepos(name)

			for idx := range repos {
				catcher.Add(queue.Put(ctx, units.NewRepoCloneJob(&repos[idx])))
			}

			if catcher.HasErrors() {
				return errors.Wrap(catcher.Resolve(), "problem queuing jobs")
			}

			amboy.WaitInterval(ctx, queue, 10*time.Millisecond)
			catcher.Add(amboy.ResolveErrors(ctx, queue))
			return catcher.Resolve()
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
				Usage: "specify a local repository, or tag to report the status of",
			},
		},
		Before: setAllTailArguements(repoFlagName),
		Action: func(c *cli.Context) error {
			tags := c.StringSlice(repoFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			var repos []sardis.RepoConf
			if len(tags) == 0 {
				// all repos
				repos = env.Configuration().Repo
			} else {
				for _, tag := range tags {
					repos = append(repos, env.Configuration().GetTaggedRepos(tag)...)
				}
			}
			catcher := grip.NewBasicCatcher()

			for _, repo := range repos {
				j := units.NewRepoStatusJob(repo.Path)
				j.Run(ctx)
				catcher.Add(j.Error())
			}

			return catcher.Resolve()
		},
	}
}

func repoFetch() cli.Command {
	const repoFlagName = "repo"
	return cli.Command{
		Name:   "fetch",
		Usage:  "fetch one or more repos",
		Before: setAllTailArguements(repoFlagName),
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  repoFlagName,
				Usage: "specify a local repository, or tag, to cleanup",
			},
		},
		Action: func(c *cli.Context) error {
			names := c.StringSlice(repoFlagName)

			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			var repos []sardis.RepoConf
			for _, tag := range names {
				repos = append(repos, env.Configuration().GetTaggedRepos(tag)...)
			}

			queue := env.Queue()

			catcher := grip.NewBasicCatcher()

			for idx := range repos {
				repo := &repos[idx]

				if repo.Fetch {
					catcher.Add(queue.Put(ctx, units.NewRepoFetchJob(repo)))
				}
			}

			amboy.WaitInterval(ctx, queue, 100*time.Millisecond)

			catcher.Add(amboy.ResolveErrors(ctx, queue))

			return catcher.Resolve()

		},
	}
}
