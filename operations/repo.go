package operations

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
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
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("list").
			Aliases("ls").
			SetUsage("return a list of configured repos"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			homedir := util.GetHomeDir()

			table := tabby.New()
			table.AddHeader("Name", "Path", "Local", "Enabled", "Tags")

			var repos []repo.GitRepository

			if len(args.arg) == 0 {
				repos = args.conf.Repos.GitRepos.Copy()
			} else {
				repos = args.conf.Repos.FindAll(args.arg...)
			}

			sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })

			for _, repo := range repos {
				sort.Strings(repo.Tags)
				table.AddLine(
					repo.Name,
					strings.Replace(repo.Path, homedir, "~", 1),
					util.FileExists(repo.Path),
					!repo.Disabled,
					strings.Join(repo.Tags, ", "))
			}

			table.Print()

			return nil
		},
	)
}

func repoUpdate() *cmdr.Commander {
	const opName = "repo-update"

	return addOpCommand(
		cmdr.MakeCommander().
			SetName("update").
			Aliases("sync"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			ct := &atomic.Int64{}

			repos := args.conf.Repos.FindAll(args.arg...).Stream()

			jobs := fun.Convert(fnx.MakeConverter(func(rc repo.GitRepository) fnx.Worker { ct.Add(1); return rc.UpdateJob() })).Stream(repos)

			err := subexec.TOOLS.WorkerPool(jobs).Run(ctx)
			switch {
			case ct.Load() == 0:
				return fmt.Errorf("no repositories for %s", args.arg)
			case err != nil:
				return err
			default:
				return nil
			}
		},
	)
}

func repoCleanup() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("gc").
			Aliases("cleanup").
			SetUsage("run repository cleanup"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			repos := args.conf.Repos.FindAll(args.arg...).Stream()

			ct := &atomic.Int64{}

			jobs := fun.Convert(fnx.MakeConverter(func(rc repo.GitRepository) fnx.Worker { ct.Add(1); return rc.CleanupJob() })).Stream(repos)

			err := subexec.TOOLS.WorkerPool(jobs).Run(ctx)
			switch {
			case ct.Load() == 0:
				return fmt.Errorf("no repositories for %s", args.arg)
			case err != nil:
				return err
			default:
				return nil
			}
		},
	)
}

func repoClone() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("clone").
			SetUsage("clone a repository or all matching repositories"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			repos := args.conf.Repos.FindAll(args.arg...).Stream()

			missingRepos := repos.Filter(func(rc repo.GitRepository) bool {
				if _, err := os.Stat(rc.Path); os.IsNotExist(err) {
					return true
				}
				grip.Warning(message.Fields{
					"path":    rc.Path,
					"op":      "clone",
					"outcome": "skipping",
				})
				return false
			})

			jobs := fun.Convert(fnx.MakeConverter(func(rc repo.GitRepository) fnx.Worker { return rc.CloneJob() })).Stream(missingRepos)

			return subexec.TOOLS.WorkerPool(jobs).Run(ctx)
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

func repoStatus() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("status").
			SetUsage("report on the status of repos"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			repos := args.conf.Repos.FindAll(args.arg...).Stream()

			ct := &atomic.Int64{}

			jobs := fun.Convert(fnx.MakeConverter(func(rc repo.GitRepository) fnx.Worker { ct.Add(1); return rc.StatusJob() })).Stream(repos)

			err := subexec.TOOLS.WorkerPool(jobs).Run(ctx)
			switch {
			case ct.Load() == 0:
				return fmt.Errorf("no repositories for %s", args.arg)
			case err != nil:
				return err
			default:
				return nil
			}
		},
	)
}

func repoFetch() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fetch").
			SetUsage("fetch one or more repos"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			repos := args.conf.Repos.FindAll(args.arg...).Stream().
				Filter(func(repo repo.GitRepository) bool { return repo.Fetch })

			ct := &atomic.Int64{}
			jobs := fun.Convert(fnx.MakeConverter(func(rc repo.GitRepository) fnx.Worker { ct.Add(1); return rc.FetchJob() })).Stream(repos)

			err := subexec.TOOLS.WorkerPool(jobs).Run(ctx)
			switch {
			case ct.Load() == 0:
				return fmt.Errorf("no repositories for %s", args.arg)
			case err != nil:
				return err
			default:
				return nil
			}
		})
}

func repoGithubClone() *cmdr.Commander {
	return cmdr.MakeCommander().
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
}
