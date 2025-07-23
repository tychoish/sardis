package repo

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

func (conf *GitRepository) FetchJob() fun.Worker {
	const opName = "repo-fetch"
	return func(ctx context.Context) (err error) {
		// double check this because we might have a stale
		// version of the config
		if err := conf.Validate(); err != nil {
			return err
		}

		id := fmt.Sprintf("fetch.REPO(%s).REMOTE(%s).OPERATOR(%s)", conf.Name, conf.RemoteName, util.GetHostname())

		startAt := time.Now()
		hostname := util.GetHostname()

		if !util.FileExists(conf.Path) {
			grip.Info(message.BuildPair().
				Pair("op", opName).
				Pair("id", id).
				Pair("repo", conf.Name).
				Pair("path", conf.Path).
				Pair("msg", "repo doesn't exist; cloning").
				Pair("host", hostname),
			)

			return conf.CloneJob().Run(ctx)
		}

		if conf.RemoteName == "" || conf.Branch == "" {
			return errors.New("repo-fetch requires defined remote name and branch for the repo")
		}

		proclog, procbuf := subexec.NewOutputBuf(id)
		defer procbuf.Close()
		proclog.Infoln(ruler, id, ruler)

		return jasper.Context(ctx).
			CreateCommand(ctx).
			ID(id).
			SetOutputSender(level.Info, procbuf).
			SetErrorSender(level.Info, procbuf).
			Directory(conf.Path).
			Append(conf.Pre...).
			AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", conf.RemoteName, conf.Branch).
			Append(conf.Post...).
			Worker().
			PreHook(func(context.Context) {
				grip.Notice(message.BuildPair().
					Pair("op", opName).
					Pair("state", "STARTED").
					Pair("repo", conf.Name).
					Pair("path", conf.Path).
					Pair("host", hostname),
				)
			}).
			WithErrorFilter(func(err error) error {
				proclog.Infoln(ruler, id, ruler)
				msg := message.BuildPair().
					Pair("op", opName).
					Pair("state", "COMPLETED").
					Pair("err", err != nil).
					Pair("dur", time.Since(startAt)).
					Pair("repo", conf.Name).
					Pair("path", conf.Path)

				if err != nil {
					grip.Error(procbuf.String())
					grip.Critical(msg.Pair("err", err))
					return err
				} else if conf.Logs.Full() {
					grip.Info(procbuf.String())
				}
				grip.Notice(msg)
				return nil
			}).PostHook(fn.MakeFuture(procbuf.Close).Ignore()).Run(ctx)
	}
}

func (conf *GitRepository) CloneJob() fun.Worker {
	const opName = "repo-clone"

	return func(ctx context.Context) error {
		hostname := util.GetHostname()
		startAt := time.Now()

		if _, err := os.Stat(conf.Path); !os.IsNotExist(err) {
			grip.Info(message.Fields{
				"op":   opName,
				"msg":  "repo exists, skipping clone, running update jobs",
				"path": conf.Path,
				"repo": conf.Remote,
				"host": hostname,
			})

			if conf.LocalSync {
				rconfCopy := conf
				rconfCopy.Pre = nil
				rconfCopy.Post = nil

				return rconfCopy.SyncRemoteJob(hostname).Run(ctx)
			}

			if conf.Fetch {
				return conf.FetchJob().Run(ctx)
			}

			return nil
		}

		sender := send.MakeAnnotating(grip.Sender(), map[string]any{
			"op":   opName,
			"repo": conf.Name,
			"host": hostname,
		})

		err := jasper.Context(ctx).CreateCommand(ctx).
			ID(fmt.Sprintf("%s.%s.clone", hostname, conf.Name)).
			Priority(level.Debug).
			Directory(filepath.Dir(conf.Path)).
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Warning, sender).
			AppendArgs("git", "clone", conf.Remote, conf.Path).
			Append(conf.Post...).
			Run(ctx)

		msg := message.BuildPair().
			Pair("op", opName).
			Pair("host", hostname).
			Pair("dur", time.Since(startAt)).
			Pair("err", err != nil).
			Pair("repo", conf.Name).
			Pair("path", conf.Path).
			Pair("remote", conf.Remote)

		if err != nil {
			grip.Error(message.WrapError(err, msg))
			return err
		}

		grip.Notice(msg)

		return nil
	}
}

func (conf *GitRepository) UpdateJob() fun.Worker {
	const opName = "repo-update"
	count := &atomic.Int64{}
	return func(ctx context.Context) error {
		wg := &fun.WaitGroup{}
		ec := &erc.Collector{}

		host := util.GetHostname()

		hasMirrors := false
		startAt := time.Now()

		id := fmt.Sprintf("%s[%d]", strings.ToLower(string(rand.Text()[:8])), count.Add(1))

		if err := conf.Validate(); err != nil {
			return ers.Wrap(err, id)
		}

		grip.Info(message.BuildPair().
			Pair("op", opName).
			Pair("state", "STARTED").
			Pair("id", id).
			Pair("path", conf.Path).
			Pair("operator", host),
		)

		defer func() {
			grip.Info(message.BuildPair().
				Pair("op", opName).
				Pair("state", "COMPLETED").
				Pair("dur", time.Since(startAt)).
				Pair("id", id).
				Pair("path", conf.Path).
				Pair("operator", host),
			)
		}()

		for _, mirror := range conf.Mirrors {
			if strings.Contains(mirror, host) {
				grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
					conf.Path, mirror, host)
				continue
			}

			hasMirrors = true
			wg.Launch(ctx, conf.SyncRemoteJob(mirror).Operation(ec.Push))
		}

		wg.Worker().Operation(ec.Push).Run(ctx)

		if !ec.Ok() {
			return ec.Resolve()
		}

		if conf.LocalSync {
			if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
				conf.FetchJob().Operation(ec.Push).Run(ctx)
				return ec.Resolve()
			}

			if changes, err := conf.HasChanges(); changes || err != nil {
				grip.Warning(message.WrapError(err, dt.Map[string, any]{
					"op": opName, "id": id, "operator": host,
					"msg": "problem detecting changes in the repository, may be routine, recoverable",
				}))

				conf.SyncRemoteJob(host).Operation(ec.Push).Run(ctx)
				return ec.Resolve()
			}
		}

		if conf.Fetch || hasMirrors || conf.LocalSync {
			conf.FetchJob().Operation(ec.Push).Run(ctx)
			return ec.Resolve()
		}

		return nil
	}
}

const (
	remoteUpdateCmdTemplate = "git add -A && git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
	ruler                   = "---------"
)

// this "remote" in the sense of a git remote, which means it might be
// the local repository in some cases
func (conf *GitRepository) SyncRemoteJob(host string) fun.Worker {
	hn := util.GetHostname()
	if host == "LOCAL" {
		host = hn
	}

	isLocal := host == hn
	var buildID string
	if isLocal {
		buildID = fmt.Sprintf("sync.LOCAL(%s).REPO(%s)", hn, conf.Name)
	} else {
		buildID = fmt.Sprintf("sync.REMOTE(%s).REPO(%s).OPERATOR(%s)", host, conf.Name, hn)
	}

	bullet := fmt.Sprintf("%s.PATH(%s)", buildID, conf.Path)

	const opName = "repo-sync"
	return func(ctx context.Context) error {
		// double check this because we might have a stale
		// version of the config
		if err := conf.Validate(); err != nil {
			return ers.Wrap(err, bullet)
		}

		if host != hn && !slices.Contains(conf.Mirrors, host) {
			return fmt.Errorf("%s: remote %q is not a configured", bullet, host)
		}

		if stat, err := os.Stat(conf.Path); os.IsNotExist(err) {
			return fmt.Errorf("path '%s' for %q does not exist", conf.Path, buildID)
		} else if !stat.IsDir() {
			return fmt.Errorf("path '%s' for %q exists but is a %s", conf.Path, buildID, stat.Mode().String())
		}
		started := time.Now()

		proclog, procbuf := subexec.NewOutputBuf(buildID)
		defer procbuf.Close()
		proclog.Noticeln(ruler, bullet, ruler)

		grip.Info(message.BuildPair().
			Pair("op", opName).
			Pair("state", "STARTED").
			Pair("id", buildID).
			Pair("path", conf.Path).
			Pair("host", host).
			Pair("operator", hn),
		)

		err := jasper.Context(ctx).
			CreateCommand(ctx).
			SetOutputSender(level.Info, procbuf).
			SetErrorSender(level.Error, procbuf).
			ID(buildID).
			Directory(conf.Path).
			AppendArgsWhen(ft.Not(isLocal), "ssh", host, fmt.Sprintf("cd %s && %s", conf.Path, fmt.Sprintf(syncCmdTemplate, buildID))).
			Append(conf.Pre...).
			AppendArgs("git", "add", "-A").
			Bash("git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
			Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
			AppendArgs("git", "add", "-A").
			Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", buildID)).
			AppendArgs("git", "push").
			AppendArgsWhen(ft.Not(isLocal), "ssh", host, fmt.Sprintf("cd %s && %s", conf.Path, fmt.Sprintf(syncCmdTemplate, buildID))).
			BashWhen(ft.Not(isLocal), "git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
			Append(conf.Post...).
			Worker().
			WithErrorFilter(func(err error) error {
				proclog.Noticeln(ruler, bullet, ruler)
				if err != nil {
					grip.Critical(message.BuildPair().
						Pair("op", opName).
						Pair("state", "ERRORED").
						Pair("host", host).
						Pair("operator", hn).
						Pair("repo", conf.Name).
						Pair("path", conf.Path).
						Pair("id", buildID).
						Pair("dur", time.Since(started)).
						Pair("err", err),
					)
					grip.Error(procbuf.String())
				} else if conf.Logs.Full() {
					grip.Info(procbuf.String())
				}
				return err
			}).
			Run(ctx)

		grip.Info(message.BuildPair().
			Pair("op", opName).
			Pair("state", "COMPLETED").
			Pair("err", err != nil).
			Pair("host", host).
			Pair("id", buildID).
			Pair("path", conf.Path),
		)

		return nil
	}
}

func (conf *GitRepository) CleanupJob() fun.Worker {
	const opName = "repo-cleanup"
	return func(ctx context.Context) (err error) {
		if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
			return fmt.Errorf("cannot cleanup %s, no repository exists", conf.Path)
		}
		id := fmt.Sprintf("cleanup.REPO(%s).OPERATOR(%s)", conf.Name, util.GetHostname())

		start := time.Now()

		defer func() {
			grip.Critical(message.BuildPair().
				Pair("op", opName).
				Pair("id", id).
				Pair("repo", conf.Name).
				Pair("path", conf.Path).
				Pair("dur", time.Since(start)).
				Pair("err", err != nil),
			)
		}()

		proclog, procbuf := subexec.NewOutputBuf(id)
		defer procbuf.Close()
		proclog.Infoln(ruler, id, ruler)

		return jasper.Context(ctx).CreateCommand(ctx).Priority(level.Info).
			Directory(conf.Path).
			SetOutputSender(level.Info, procbuf).
			SetErrorSender(level.Error, procbuf).
			AppendArgs("git", "gc").
			AppendArgs("git", "prune").
			Worker().
			WithErrorFilter(func(err error) error {
				proclog.Infoln(ruler, id, ruler)
				if err != nil {
					grip.Critical(message.BuildPair().
						Pair("op", opName).
						Pair("id", id).
						Pair("repo", conf.Name).
						Pair("path", conf.Path).
						Pair("err", err),
					)
					grip.Error(procbuf.String())
				} else if conf.Logs.Full() {
					grip.Info(procbuf.String())
				}

				return err
			}).PostHook(fn.MakeFuture(procbuf.Close).Ignore()).Run(ctx)
	}
}

func (conf *GitRepository) StatusJob() fun.Worker {
	return func(ctx context.Context) error {
		if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
			return fmt.Errorf("cannot check status %s, no repository exists", conf.Path)
		}

		cmd := jasper.Context(ctx)

		startAt := time.Now()

		logger := grip.Context(ctx)
		sender := logger.Sender()

		ec := &erc.Collector{}
		ec.Add(cmd.CreateCommand(ctx).Priority(level.Debug).
			Directory(conf.Path).
			SetOutputSender(level.Debug, sender).
			SetErrorSender(level.Debug, sender).
			Add(conf.getStatusCommandArgs()).
			AppendArgs("git", "status", "--short", "--branch").
			Run(ctx))

		ec.Add(conf.doOtherStat(logger))

		logger.Debug(message.Fields{
			"op":   "git status",
			"path": conf.Path,
			"dur":  time.Since(startAt).Seconds(),
			"ok":   ec.Ok(),
		})

		return ec.Resolve()
	}
}

func (conf *GitRepository) getStatusCommandArgs() []string {
	return []string{
		"git", "log", "--date=relative", "--decorate", "-n", "1",
		fmt.Sprint("--format=", filepath.Base(conf.Path)), `:%N (%cr) "%s"`,
	}
}

func (conf *GitRepository) doOtherStat(logger grip.Logger) error {
	repo, err := git.PlainOpen(conf.Path)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	stat, err := wt.Status()
	if err != nil {
		return err
	}

	for fn, status := range stat {
		logger.Notice(message.Fields{
			"file":     fn,
			"stat":     "golib",
			"staging":  status.Staging,
			"worktree": status.Worktree,
		})
	}
	return nil
}
