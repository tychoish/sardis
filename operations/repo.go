package operations

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/repo"
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
			SetUsage("return a list of configured repos").
			Subcommanders(addOpCommand(
				cmdr.MakeCommander().
					SetName("tags").
					SetUsage("list repos by tag groups"),
				"tag", func(ctx context.Context, arg *opsCmdArgs[[]string]) error {
					table := tabby.New()
					tags := arg.conf.RepoTags()

					matcher := dt.NewSetFromSlice(arg.ops)
					table.AddHeader("Tag", "Count", "Repositories")
					err := tags.Stream().Filter(func(tp dt.Pair[string, dt.Slice[*repo.Configuration]]) bool {

						// if this is a "list all" situation then only give us the tags; but if we
						// ask for a specific term, give us the full match.
						return (matcher.Len() == 0 && (len(tp.Value) > 1 || tp.Value[0].Name != tp.Key)) ||
							matcher.Check(tp.Key)

					}).ReadAll(func(rpt dt.Pair[string, dt.Slice[*repo.Configuration]]) {
						rns, err := fun.MakeConverter(func(r *repo.Configuration) string { return r.Name }).
							Stream(dt.NewSlice(rpt.Value).Stream()).Slice(ctx)
						grip.Error(message.WrapError(err, "problem rendering repo names"))

						subsequent := false
						for chunk := range slices.Chunk(rns, 5) {
							if !subsequent {
								table.AddLine(rpt.Key, len(rns), strings.Join(chunk, ", "))
								subsequent = true
								continue
							}
							table.AddLine("", "", strings.Join(chunk, ", "))
						}
						table.AddLine("", "", "")
					}).Run(ctx)

					if err != nil {
						return ers.Wrap(err, "problem finding repo tags for table")
					}

					table.Print()
					return nil
				},
			)),
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			homedir := util.GetHomedir()

			table := tabby.New()
			table.AddHeader("Name", "Path", "Local", "Enabled", "Tags")

			var repos []repo.Configuration

			if len(args.ops) == 0 {
				repos = args.conf.Repo
			} else {
				repos = args.conf.GetTaggedRepos(args.ops...)
			}

			sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })

			for _, repo := range repos {
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
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			started := time.Now()

			repos := args.conf.GetTaggedRepos(args.ops...)
			if len(repos) == 0 {
				return fmt.Errorf("no tagged repository named '%v' configured", args.ops)
			}

			shouldNotify := false
			repoNames := make([]string, 0, len(args.ops))
			filterd := repos.Stream().Filter(func(conf repo.Configuration) bool {
				shouldNotify = shouldNotify || conf.Disabled && conf.Notify
				if ft.Not(conf.Disabled) {
					repoNames = append(repoNames, conf.Name)
					return true
				}
				return false
			})

			var err error
			jobs := fun.MakeConverter(func(rc repo.Configuration) fun.Worker { return rc.FullSync() }).Stream(filterd)

			grip.Info(message.BuildPair().
				Pair("op", opName).
				Pair("state", "starting").
				Pair("ops", args.ops).
				Pair("host", args.conf.Settings.Runtime.Hostname),
			)
			defer func() {
				// QUESTION: should we send notification here
				grip.Notice(message.BuildPair().
					Pair("op", opName).
					Pair("err", ers.IsError(err)).
					Pair("state", "complete").
					Pair("host", args.conf.Settings.Runtime.Hostname).
					Pair("args", args.ops).
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
					"arg":   args.ops,
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
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			repos := args.conf.GetTaggedRepos(args.ops...).Stream()

			jobs := fun.MakeConverter(func(rc repo.Configuration) fun.Worker { return rc.CleanupJob() }).Stream(repos)

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
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			repos := args.conf.GetTaggedRepos(args.ops...).Stream()

			missingRepos := repos.Filter(func(rc repo.Configuration) bool {
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

			jobs := fun.MakeConverter(func(rc repo.Configuration) fun.Worker { return rc.CloneJob() }).Stream(missingRepos)

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
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			ec := &erc.Collector{}

			repos := args.conf.GetTaggedRepos(args.ops...).Stream()

			jobs := fun.MakeConverter(func(rc repo.Configuration) fun.Worker { grip.Info(rc.Name); return rc.StatusJob() }).Stream(repos)

			ops := fun.MakeConverter(func(wf fun.Worker) fun.Operation { return wf.Operation(ec.Push) }).Stream(jobs)

			ec.Push(ops.ReadAll(func(op fun.Operation) { op.Run(ctx) }).Run(ctx))

			return ec.Resolve()
		},
	)
}

func repoFetch() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fetch").
			SetUsage("fetch one or more repos"),
		"repo", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			repos := args.conf.GetTaggedRepos(args.ops...).Stream().Filter(func(repo repo.Configuration) bool { return repo.Fetch })

			jobs := fun.MakeConverter(func(rc repo.Configuration) fun.Worker { return rc.FetchJob() }).Stream(repos)

			return jobs.Parallel(
				func(ctx context.Context, op fun.Worker) error { return op(ctx) },
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfWorkerPerCPU(),
			).Run(ctx)
		})
}
