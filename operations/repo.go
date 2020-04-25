package operations

import (
	"os"
	"strings"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Repo() cli.Command {
	return cli.Command{
		Name:  "repo",
		Usage: "a collections of commands to manage repositories",
		Subcommands: []cli.Command{
			updateRepos(),
		},
	}
}

func updateRepos() cli.Command {
	hostname, err := os.Hostname()
	grip.Warning(err)

	return cli.Command{
		Name:   "update",
		Usage:  "update a local and remote git repository according to the config",
		Before: requireConfig(),
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "repo",
				Usage: "specify a local repository to udpate",
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

			for _, repo := range conf.Repo {
				if repoName != "" && repo.Name != repoName {
					continue
				}

				if repo.LocalSync {
					catcher.Add(queue.Put(ctx, units.NewLocalRepoSyncJob(repo.Path)))
				} else if repo.Fetch {
					catcher.Add(queue.Put(ctx, units.NewRepoFetchJob(repo)))
				}

				for _, mirror := range repo.Mirrors {
					if strings.Contains(mirror, hostname) {
						grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
							repo.Path, mirror, hostname)
						continue
					}
					catcher.Add(queue.Put(ctx, units.NewRepoSyncRemoteJob(mirror, repo.Path, repo.Pre, repo.Post)))
				}
			}

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
