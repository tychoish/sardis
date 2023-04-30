package operations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/amboy"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
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
			repoList(),
		},
	}
}

func repoList() cli.Command {
	return cli.Command{
		Name:  "list",
		Usage: "return a list of configured repos",
		Action: func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			homedir, _ := homedir.Expand("~/")

			table := tabby.New()
			table.AddHeader("Name", "Path", "Local", "Enabled", "Tags")
			for _, repo := range conf.Repo {
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

func repoUpdate() cli.Command {
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
		Before: setAllTailArguements(repoTagFlagName),
		Action: func(ctx context.Context, c *cli.Context) error {
			tags := c.StringSlice(repoTagFlagName)

			conf := sardis.AppConfiguration(ctx)
			ctx = sardis.WithRemoteNotify(ctx, conf)
			notify := sardis.RemoteNotify(ctx)

			repos := conf.GetTaggedRepos(tags...)
			if len(repos) == 0 {
				return fmt.Errorf("no tagged repository named '%v' configured", tags)
			}

			shouldNotify := false

			started := time.Now()
			jobs, worker := units.SetupWorkers()
			for idx := range repos {
				repo := repos[idx]
				if repo.Disabled {
					continue
				}
				if repo.Notify {
					shouldNotify = true
				}
				jobs.PushBack(units.SyncRepo(repo))
			}

			grip.Info(message.Fields{
				"op":      "repo sync",
				"message": "waiting for jobs to complete",
				"tags":    tags,
			})

			if err := worker(ctx); err != nil {
				return err
			}

			if shouldNotify {
				notify.Notice(message.Fields{
					"tag":     tags,
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
				"repos":   len(repos),
			})

			return nil
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
		Action: func(ctx context.Context, c *cli.Context) error {
			tags := c.StringSlice(repoFlagName)

			repos := sardis.AppConfiguration(ctx).GetTaggedRepos(tags...)

			jobs, run := units.SetupQueue(amboy.RunJob)

			for _, repo := range repos {
				jobs.PushBack(units.NewRepoCleanupJob(repo.Path))
			}

			return run(ctx)
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
		Action: func(ctx context.Context, c *cli.Context) error {
			name := c.String(nameFlagName)

			conf := sardis.AppConfiguration(ctx)

			repos := conf.GetTaggedRepos(name)
			jobs, run := units.SetupWorkers()

			for idx := range repos {
				if _, err := os.Stat(repos[idx].Path); !os.IsNotExist(err) {
					grip.Warning(message.Fields{
						"path":    repos[idx].Path,
						"op":      "clone",
						"outcome": "skipping",
					})
					continue
				}

				jobs.PushBack(units.NewRepoCloneJob(repos[idx]))
			}

			return run(ctx)
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
		Action: func(ctx context.Context, c *cli.Context) error {
			tags := c.StringSlice(repoFlagName)

			conf := sardis.AppConfiguration(ctx)

			catcher := &erc.Collector{}

			catcher.Add(fun.Observe(ctx,
				itertool.Slice(conf.GetTaggedRepos(tags...)),
				func(conf sardis.RepoConf) {
					grip.Info(conf.Name)
					catcher.Add(units.WorkerJob(units.NewRepoStatusJob(conf.Path)).Run(ctx))
				}))

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
		Action: func(ctx context.Context, c *cli.Context) error {
			names := c.StringSlice(repoFlagName)

			repos := sardis.AppConfiguration(ctx).GetTaggedRepos(names...)

			jobs, run := units.SetupQueue(amboy.RunJob)
			for idx := range repos {
				repo := repos[idx]

				if repo.Fetch {
					jobs.PushBack(units.NewRepoFetchJob(repo))

				}
			}

			return run(ctx)

		},
	}
}
