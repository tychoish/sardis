package operations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Repo(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "a collections of commands to manage repositories",
		Subcommands: []cli.Command{
			repoClone(ctx),
			repoUpdate(ctx),
			repoCleanup(ctx),
			repoStatus(ctx),
			repoFetch(ctx),
			repoList(ctx),
		},
	}
}

func repoList(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "return a list of configured repos",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)

			homedir, _ := homedir.Expand("~/")

			table := tabby.New()
			table.AddHeader("Name", "Path", "Local", "Enabled", "Tags")
			for _, repo := range env.Configuration().Repo {
				_, err := os.Stat(repo.Path)
				fileExists := !os.IsNotExist(err)
				table.AddLine(
					repo.Name,
					strings.Replace(repo.Path, homedir, "~", 1),
					fileExists,
					repo.LocalSync || repo.Fetch,
					repo.Tags)
			}

			table.Print()

			return nil
		},
	}
}

func repoUpdate(ctx context.Context) cli.Command {
	const repoTagFlagName = "repo"
	return cli.Command{
		Name:    "update",
		Aliases: []string{"sync"},
		Usage:   "update a local and remote git repository according to the config",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  repoTagFlagName,
				Usage: "specify tag of repos to update",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(ctx), setAllTailArguements(repoTagFlagName)),
		Action: func(c *cli.Context) error {
			tags := c.StringSlice(repoTagFlagName)

			env := sardis.GetEnvironment(ctx)
			ctx, cancel := env.Context()
			defer cancel()

			notify := env.Logger()
			conf := env.Configuration()

			repos := conf.GetTaggedRepos(tags...)
			if len(repos) == 0 {
				return fmt.Errorf("no tagged repository named '%v' configured", tags)
			}

			queue := env.Queue()
			catcher := &erc.Collector{}
			wg := &sync.WaitGroup{}

			shouldNotify := false

			for idx := range repos {
				repo := &repos[idx]
				if repo.Disabled {
					continue
				}
				if repo.Notify {
					shouldNotify = true
				}
				units.SyncRepo(ctx, catcher, wg, queue, repo)
			}

			wg.Wait()

			started := time.Now()
			stat := queue.Stats(ctx)
			grip.Info(message.Fields{
				"op":      "repo sync",
				"message": "waiting for jobs to complete",
				"jobs":    stat.Total,
				"tags":    tags,
			})
			if stat.Total > 0 || !stat.IsComplete() {
				amboy.WaitInterval(ctx, queue, 10*time.Millisecond)
			}
			catcher.Add(amboy.ResolveErrors(ctx, queue))

			if shouldNotify {
				notify.Notice(message.Fields{
					"tag":     tags,
					"errors":  catcher.HasErrors(),
					"jobs":    stat.Total,
					"repos":   len(repos),
					"op":      "repo sync",
					"dur_sec": time.Since(started).Seconds(),
				})
			}

			// QUESTION: should we send notification here
			grip.Notice(message.Fields{
				"op":      "repo sync",
				"code":    "success",
				"tag":     tags,
				"dur_sec": time.Since(started).Seconds(),
				"err":     catcher.HasErrors(),
				"jobs":    stat.Total,
			})

			return catcher.Resolve()
		},
	}
}

func repoCleanup(ctx context.Context) cli.Command {
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

			env := sardis.GetEnvironment(ctx)
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
			catcher := &erc.Collector{}
			for _, repo := range repos {
				catcher.Add(queue.Put(ctx, units.NewRepoCleanupJob(repo.Path)))
			}

			amboy.WaitInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}

func repoClone(ctx context.Context) cli.Command {
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

			env := sardis.GetEnvironment(ctx)
			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()

			catcher := &erc.Collector{}
			queue := env.Queue()

			repos := conf.GetTaggedRepos(name)

			for idx := range repos {
				if _, err := os.Stat(repos[idx].Path); !os.IsNotExist(err) {
					grip.Warning(message.Fields{
						"path":    repos[idx].Path,
						"op":      "clone",
						"outcome": "skipping",
					})
					continue
				}

				catcher.Add(queue.Put(ctx, units.NewRepoCloneJob(&repos[idx])))
			}

			if catcher.HasErrors() {
				return fmt.Errorf("problem queuing jobs: %w", catcher.Resolve())
			}

			amboy.WaitInterval(ctx, queue, 10*time.Millisecond)
			catcher.Add(amboy.ResolveErrors(ctx, queue))
			return catcher.Resolve()
		},
	}

}

func repoStatus(ctx context.Context) cli.Command {
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

			env := sardis.GetEnvironment(ctx)
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
			catcher := &erc.Collector{}

			for _, repo := range repos {
				j := units.NewRepoStatusJob(repo.Path)
				j.Run(ctx)
				catcher.Add(j.Error())
			}

			return catcher.Resolve()
		},
	}
}

func repoFetch(ctx context.Context) cli.Command {
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

			env := sardis.GetEnvironment(ctx)
			ctx, cancel := env.Context()
			defer cancel()

			var repos []sardis.RepoConf
			for _, tag := range names {
				repos = append(repos, env.Configuration().GetTaggedRepos(tag)...)
			}

			queue := env.Queue()

			catcher := &erc.Collector{}

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
