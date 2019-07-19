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
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer env.Close(ctx)
			defer cancel()

			queue := env.Queue()
			conf := env.Configuration()
			catcher := grip.NewBasicCatcher()

			for _, repo := range conf.Repo {
				if repo.LocalSync {
					catcher.Add(queue.Put(units.NewLocalRepoSyncJob(repo.Path)))
				} else if repo.Fetch {
					catcher.Add(queue.Put(units.NewRepoFetchJob(repo)))
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

			for _, link := range conf.Links {
				catcher.Add(queue.Put(units.NewSymlinkCreateJob(link)))
			}

			if catcher.HasErrors() {
				return catcher.Resolve()
			}

			amboy.WaitCtxInterval(ctx, queue, time.Millisecond)

			return amboy.ResolveErrors(ctx, queue)
		},
	}
}
