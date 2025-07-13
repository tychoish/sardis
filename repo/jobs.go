package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
)

func (conf *Configuration) FetchJob() fun.Worker {
	return func(ctx context.Context) (err error) {
		start := time.Now()
		defer func() {
			grip.Info(message.Fields{
				"op":    "repo-fetch",
				"path":  conf.Path,
				"err":   err != nil,
				"repo":  conf.Remote,
				"dur":   time.Since(start).String(),
				"host":  util.GetHostname(),
				"dir":   conf.Path,
				"npre":  len(conf.Pre),
				"npost": len(conf.Post),
			})
		}()

		if !util.FileExists(conf.Path) {
			grip.Info(message.Fields{
				"path": conf.Path,
				"op":   "repo doesn't exist; cloning",
			})

			return conf.CloneJob().Run(ctx)
		}

		if conf.RemoteName == "" || conf.Branch == "" {
			return errors.New("repo-fetch requires defined remote name and branch for the repo")
		}

		cmd := jasper.Context(ctx).
			CreateCommand(ctx).
			Directory(conf.Path)
			// SetOutputSender(level.Trace, sender).
			// SetErrorSender(level.Debug, sender)

		if conf.LocalSync && slices.Contains(conf.Tags, "mail") {
			cmd.Append(conf.Pre...)
		}

		cmd.AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", conf.RemoteName, conf.Branch)
		cmd.Append(conf.Post...)

		return cmd.Run(ctx)
	}
}

func (conf *Configuration) CloneJob() fun.Worker {
	const opName = "repo-clone"

	return func(ctx context.Context) error {
		hostname := util.GetHostname()
		startAt := time.Now()

		if _, err := os.Stat(conf.Path); !os.IsNotExist(err) {
			grip.Info(message.Fields{
				"op":   opName,
				"msg":  "repo exists, skipping clone, running update jobs as needed",
				"path": conf.Path,
				"repo": conf.Remote,
				"host": hostname,
			})

			if conf.LocalSync {
				rconfCopy := conf
				rconfCopy.Pre = nil
				rconfCopy.Post = nil
				return rconfCopy.Sync(hostname).Run(ctx)
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

func (conf *Configuration) FullSync() fun.Worker {
	return func(ctx context.Context) error {
		wg := &fun.WaitGroup{}
		ec := &erc.Collector{}

		hostname := util.GetHostname()

		hasMirrors := false

		for _, mirror := range conf.Mirrors {
			if strings.Contains(mirror, hostname) {
				grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
					conf.Path, mirror, hostname)
				continue
			}

			hasMirrors = true
			wg.Launch(ctx, conf.Sync(mirror).Operation(ec.Push))
		}

		wg.Worker().Operation(ec.Push).Run(ctx)

		if !ec.Ok() {
			return ec.Resolve()
		}

		if conf.LocalSync {
			if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
				return conf.FetchJob().Run(ctx)
			}

			if changes, err := conf.HasChanges(); changes || err != nil {
				return conf.Sync(hostname).Run(ctx)
			}
		}

		if conf.Fetch || hasMirrors || conf.LocalSync {
			return conf.FetchJob().Run(ctx)
		}

		return nil
	}
}

const (
	remoteUpdateCmdTemplate = "git add -A && git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
	ruler                   = "---------"
)

func (conf *Configuration) Sync(host string) fun.Worker {
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

	return func(ctx context.Context) error {
		if stat, err := os.Stat(conf.Path); os.IsNotExist(err) {
			return fmt.Errorf("path '%s' for %q does not exist", conf.Path, buildID)
		} else if !stat.IsDir() {
			return fmt.Errorf("path '%s' for %q exists but is a %s", conf.Path, buildID, stat.Mode().String())
		}
		started := time.Now()

		procout := &bufsend{}
		procout.SetPriority(level.Info)
		procout.SetName(buildID)
		procout.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))
		proclog := grip.NewLogger(procout)
		proclog.Noticeln(
			ruler,
			"repo:", strings.ToUpper(conf.Name), "---",
			"host:", strings.ToUpper(host), "---",
			"path:", strings.ToUpper(conf.Path),
			ruler,
		)

		grip.Info(message.BuildPair().
			Pair("op", "repo-sync").
			Pair("state", "started").
			Pair("id", buildID).
			Pair("path", conf.Path).
			Pair("host", host),
		)

		err := jasper.Context(ctx).
			CreateCommand(ctx).
			SetOutputSender(level.Info, procout).
			SetErrorSender(level.Info, procout).
			Priority(level.Debug).
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
			Run(ctx)

		msg := message.BuildPair().
			Pair("op", "repo-sync").
			Pair("state", "completed").
			Pair("host", host).
			Pair("errors", err != nil).
			Pair("id", buildID).
			Pair("path", conf.Path).
			Pair("dur", time.Since(started))

		if err != nil {
			proclog.Noticeln(
				ruler,
				"repo:", strings.ToUpper(conf.Name), "----",
				"host:", strings.ToUpper(host), "----",
				"path:", strings.ToUpper(conf.Path),
				ruler,
			)

			grip.Error(msg)
			grip.Error(procout.buffer.String())

			return err
		}

		grip.Info(msg)
		return nil
	}
}

func (conf *Configuration) StatusJob() fun.Worker {
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

func (conf *Configuration) getStatusCommandArgs() []string {
	return []string{
		"git", "log", "--date=relative", "--decorate", "-n", "1",
		fmt.Sprint("--format=", filepath.Base(conf.Path)), `:%N (%cr) "%s"`,
	}
}

func (conf *Configuration) doOtherStat(logger grip.Logger) error {
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
