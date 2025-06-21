package operations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
)

func Repo() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("repo").
		SetUsage("a collections of commands to manage repositories").
		Subcommanders(
			repoList(),
			repoUpdate(),
			repoClone(),
			repoGithubClone(),
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
			homedir := util.GetHomedir()

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

func repoUpdate() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("update").
		Aliases("sync")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
		repos := args.conf.GetTaggedRepos(args.ops...)
		if len(repos) == 0 {
			return fmt.Errorf("no tagged repository named '%v' configured", args.ops)
		}

		notify := sardis.RemoteNotify(ctx)

		shouldNotify := false

		started := time.Now()
		ec := &erc.Collector{}
		jobs, run := units.SetupWorkers(ec)

		repoNames := make([]string, 0, len(repos))
		for idx := range repos {
			repo := repos[idx]
			if repo.Disabled {
				continue
			}
			if repo.Notify {
				shouldNotify = true
			}
			jobs.PushBack(units.SyncRepo(ec, repo))
			repoNames = append(repoNames, repo.Name)
		}

		grip.Info(message.Fields{
			"op":      "repo sync",
			"message": "waiting for jobs to complete",
			"tags":    args.ops,
			"repos":   repoNames,
		})
		ec.Add(run(ctx))

		if shouldNotify && ec.Ok() {
			notify.Notice(message.Fields{
				"tag":     args.ops,
				"repos":   repoNames,
				"op":      "repo sync",
				"dur_sec": time.Since(started).Seconds(),
			})
		}

		// QUESTION: should we send notification here
		grip.Notice(message.Fields{
			"op":      "repo sync",
			"code":    "success",
			"tag":     args.ops,
			"repos":   repoNames,
			"dur_sec": time.Since(started).Seconds(),
			"ok":      ec.Ok(),
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

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
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
	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
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

func fallbackTo[T comparable](first T, args ...T) (out T) {
	if first != out {
		return first
	}
	for _, arg := range args {
		if arg != out {
			return arg
		}
	}

	return out
}

func repoGithubClone() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("gh-clone").Aliases("gh", "ghc").
		SetUsage("clone a repository or all matching repositories").
		Flags(
			cmdr.FlagBuilder("tychoish").
				SetName("account", "a").
				SetUsage("name of ").
				Flag(),
			cmdr.FlagBuilder("").
				SetName("repo", "r").
				SetUsage("name of repository").
				Flag(),
			cmdr.FlagBuilder(ft.Must(os.Getwd())).
				SetName("path", "p").
				SetUsage("path to clone repo to, defaults to pwd").
				Flag(),
		).
		SetAction(func(ctx context.Context, cc *cli.Context) error {
			jpm := jasper.Context(ctx)
			args := append(cc.Args().Slice(), "", "")

			grip.Infoln(args, len(args))

			account := fallbackTo(cc.String("account"), args...)
			repo := fallbackTo(cc.String("repo"), args[1:]...)
			repoPath := fmt.Sprintf("git@github.com:%s/%s.git", account, repo)

			grip.Notice(repoPath)

			return jpm.CreateCommand(ctx).
				Directory(cc.String("path")).
				SetCombinedSender(level.Debug, grip.Sender()).
				AppendArgs("git", "clone", repoPath).Run(ctx)
		})
	return cmd
}

func repoStatus() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("status").
		SetUsage("report on the status of repos")
	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
		catcher := &erc.Collector{}

		catcher.Add(fun.SliceStream(args.conf.GetTaggedRepos(args.ops...)).ReadAll(func(conf sardis.RepoConf) {
			grip.Info(conf.Name)
			catcher.Add(units.NewRepoStatusJob(conf.Path).Run(ctx))
		}).Run(ctx))

		return catcher.Resolve()
	})
}

func repoFetch() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("fetch").
		SetUsage("fetch one or more repos")

	return addOpCommand(cmd, "repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
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
