package operations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli/v2"
)

func Repo() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("repo").
		SetUsage("a collections of commands to manage repositories").
		Subcommanders(
			repoList(),
			repoUpdate(),
			repoClone(),
			repoCleanup(),
			repoStatus(),
			repoFetch(),
		)
}

func repoList() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("list").
		SetUsage("return a list of configured repos").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			homedir := util.GetHomeDir()

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
		}).Add)
}

type opsCmdArgs struct {
	conf *sardis.Configuration
	ops  []string
}

func addOpCommand(cmd *cmdr.Commander, name string, op func(ctx context.Context, args *opsCmdArgs) error) *cmdr.Commander {
	return cmd.Flags(cmdr.FlagBuilder([]string{}).
		SetName(name).
		SetUsage(fmt.Sprintf("specify one or more %s", name)).
		Flag(),
	).With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (*opsCmdArgs, error) {
		conf, err := ResolveConfiguration(ctx, cc)
		if err != nil {
			return nil, err
		}
		ops := append(cc.StringSlice(name), cc.Args().Slice()...)

		return &opsCmdArgs{conf: conf, ops: ops}, nil
	}).SetMiddleware(func(ctx context.Context, args *opsCmdArgs) context.Context {
		ctx = sardis.WithConfiguration(ctx, args.conf)
		return sardis.WithRemoteNotify(ctx, args.conf)
	}).SetAction(op).Add)
}

func repoUpdate() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("update").
		Aliases("sync")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs) error {
		repos := args.conf.GetTaggedRepos(args.ops...)
		if len(repos) == 0 {
			return fmt.Errorf("no tagged repository named '%v' configured", args.ops)
		}

		notify := sardis.RemoteNotify(ctx)

		shouldNotify := false

		started := time.Now()
		ec := &erc.Collector{}
		jobs, run := units.SetupWorkers(ec)

		for idx := range repos {
			repo := repos[idx]
			if repo.Disabled {
				continue
			}
			if repo.Notify {
				shouldNotify = true
			}
			jobs.PushBack(units.SyncRepo(ec, repo))
		}

		grip.Info(message.Fields{
			"op":      "repo sync",
			"message": "waiting for jobs to complete",
			"tags":    args.ops,
		})
		ec.Add(run(ctx))

		if shouldNotify && !ec.HasErrors() {
			notify.Notice(message.Fields{
				"tag":     args.ops,
				"op":      "repo sync",
				"dur_sec": time.Since(started).Seconds(),
			})
		}

		// QUESTION: should we send notification here
		grip.Notice(message.Fields{
			"op":      "repo sync",
			"code":    "success",
			"tag":     args.ops,
			"dur_sec": time.Since(started).Seconds(),
			"err":     ec.HasErrors(),
			"repos":   len(repos),
		})

		return ec.Resolve()
	})
}

func repoCleanup() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("gc").
		Aliases("cleanup").
		SetUsage("run repository cleanup")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs) error {
		repos := args.conf.GetTaggedRepos(args.ops...)
		ec := &erc.Collector{}
		jobs, run := units.SetupWorkers(ec)

		for _, repo := range repos {
			jobs.PushBack(units.NewRepoCleanupJob(repo.Path))
		}
		ec.Add(run(ctx))
		return ec.Resolve()
	})
}

func repoClone() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("clone").
		SetUsage("clone a repository or all matching repositories")
	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs) error {
		repos := args.conf.GetTaggedRepos(args.ops...)

		ec := &erc.Collector{}
		jobs, run := units.SetupWorkers(ec)

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

		ec.Add(run(ctx))
		return ec.Resolve()
	})
}

func repoStatus() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("status").
		SetUsage("report on the status of repos")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs) error {
		catcher := &erc.Collector{}

		catcher.Add(fun.Observe(ctx,
			itertool.Slice(args.conf.GetTaggedRepos(args.ops...)),
			func(conf sardis.RepoConf) {
				grip.Info(conf.Name)
				catcher.Add(units.NewRepoStatusJob(conf.Path).Run(ctx))
			}))

		return catcher.Resolve()
	})
}

func repoFetch() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("fetch").
		SetUsage("fetch one or more repos")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs) error {
		ec := &erc.Collector{}
		jobs, run := units.SetupWorkers(ec)

		repos := args.conf.GetTaggedRepos(args.ops...)

		for idx := range repos {
			repo := repos[idx]

			if repo.Fetch {
				jobs.PushBack(units.NewRepoFetchJob(repo))
			}
		}

		ec.Add(run(ctx))
		return ec.Resolve()
	})
}
