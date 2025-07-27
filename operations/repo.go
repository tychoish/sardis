package operations

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/repo"
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
			started := time.Now()

			repos := args.conf.Repos.FindAll(args.arg...)
			if len(repos) == 0 {
				return fmt.Errorf("no tagged repository named '%v' configured", args.arg)
			}

			shouldNotify := false
			repoNames := make([]string, 0, len(args.arg))
			filterd := repos.Stream().Filter(func(conf repo.GitRepository) bool {
				shouldNotify = shouldNotify || conf.Disabled && conf.Notify
				if ft.Not(conf.Disabled) {
					repoNames = append(repoNames, conf.Name)
					return true
				}
				return false
			})

			var err error
			jobs := fun.MakeConverter(func(rc repo.GitRepository) fun.Worker { return rc.UpdateJob() }).Stream(filterd)

			grip.Info(message.BuildPair().
				Pair("op", opName).
				Pair("state", "STARTED").
				Pair("ops", args.arg).
				Pair("host", args.conf.Settings.Runtime.Hostname),
			)
			defer func() {
				// QUESTION: should we send notification here
				grip.Notice(message.BuildPair().
					Pair("op", opName).
					Pair("err", ers.IsError(err)).
					Pair("state", "COMPLETED").
					Pair("host", args.conf.Settings.Runtime.Hostname).
					Pair("args", args.arg).
					Pair("repos", repoNames).
					Pair("dur", time.Since(started)),
				)
			}()

			err = jobs.Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
			if err != nil {
				sardis.RemoteNotify(ctx).NoticeWhen(shouldNotify, message.Fields{
					"arg":   args.arg,
					"repos": repoNames,
					"op":    opName,
					"dur":   time.Since(started).Seconds(),
				})
				return err
			}

			return nil
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

			jobs := fun.MakeConverter(func(rc repo.GitRepository) fun.Worker { return rc.CleanupJob() }).Stream(repos)

			return jobs.Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
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

			jobs := fun.MakeConverter(func(rc repo.GitRepository) fun.Worker { return rc.CloneJob() }).Stream(missingRepos)

			return jobs.Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
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

func repoStatus() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("status").
			SetUsage("report on the status of repos"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			ec := &erc.Collector{}

			repos := args.conf.Repos.FindAll(args.arg...).Stream()

			fun.MakeConverter(
				func(rc repo.GitRepository) fun.Operation {
					grip.Info(rc.Name)
					return rc.StatusJob().Operation(ec.Push)
				},
			).Stream(repos).ReadAll(func(op fun.Operation) { op.Run(ctx) }).Operation(ec.Push).Run(ctx)

			return ec.Resolve()
		},
	)
}

func repoFetch() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fetch").
			SetUsage("fetch one or more repos"),
		"repo", func(ctx context.Context, args *withConf[[]string]) error {
			repos := args.conf.Repos.FindAll(args.arg...).Stream().Filter(func(repo repo.GitRepository) bool { return repo.Fetch })

			jobs := fun.MakeConverter(func(rc repo.GitRepository) fun.Worker { return rc.FetchJob() }).Stream(repos)

			return jobs.Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}
