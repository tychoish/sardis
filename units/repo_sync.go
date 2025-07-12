package units

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

const (
	remoteUpdateCmdTemplate = "git add -A && git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
	ruler                   = "---------"
)

func NewLocalRepoSyncJob(repo sardis.RepoConf) fun.Worker {
	return NewRepoSyncJob("LOCAL", repo)
}

func NewRepoSyncJob(host string, repo sardis.RepoConf) fun.Worker {
	hn := util.GetHostname()
	if host == "LOCAL" {
		host = hn
	}

	isLocal := host == hn
	var buildID string
	if isLocal {
		buildID = fmt.Sprintf("sync.LOCAL(%s).REPO(%s)", hn, repo.Name)
	} else {
		buildID = fmt.Sprintf("sync.REMOTE(%s).REPO(%s).OPERATOR(%s)", host, repo.Name, hn)
	}

	return func(ctx context.Context) error {
		if stat, err := os.Stat(repo.Path); os.IsNotExist(err) {
			return fmt.Errorf("path '%s' for %q does not exist", repo.Path, buildID)
		} else if !stat.IsDir() {
			return fmt.Errorf("path '%s' for %q exists but is a %s", repo.Path, buildID, stat.Mode().String())
		}
		started := time.Now()

		procout := &bufsend{}
		procout.SetPriority(level.Info)
		procout.SetName(buildID)
		procout.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))
		proclog := grip.NewLogger(procout)
		proclog.Noticeln(
			ruler,
			"repo:", strings.ToUpper(repo.Name), "---",
			"host:", strings.ToUpper(host), "---",
			"path:", strings.ToUpper(repo.Path),
			ruler,
		)

		grip.Info(message.BuildPair().
			Pair("op", "repo-sync").
			Pair("state", "started").
			Pair("id", buildID).
			Pair("path", repo.Path).
			Pair("host", host),
		)

		err := jasper.Context(ctx).
			CreateCommand(ctx).
			SetOutputSender(level.Info, procout).
			SetErrorSender(level.Info, procout).
			Priority(level.Debug).
			ID(buildID).
			Directory(repo.Path).
			AppendArgsWhen(ft.Not(isLocal), "ssh", host, fmt.Sprintf("cd %s && %s", repo.Path, fmt.Sprintf(syncCmdTemplate, buildID))).
			Append(repo.Pre...).
			AppendArgs("git", "add", "-A").
			Bash("git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
			Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
			AppendArgs("git", "add", "-A").
			Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", buildID)).
			AppendArgs("git", "push").
			AppendArgsWhen(ft.Not(isLocal), "ssh", host, fmt.Sprintf("cd %s && %s", repo.Path, fmt.Sprintf(syncCmdTemplate, buildID))).
			BashWhen(ft.Not(isLocal), "git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
			Append(repo.Post...).
			Run(ctx)

		msg := message.BuildPair().
			Pair("op", "repo-sync").
			Pair("state", "completed").
			Pair("host", host).
			Pair("errors", err != nil).
			Pair("id", buildID).
			Pair("path", repo.Path).
			Pair("dur", time.Since(started))

		if err != nil {
			proclog.Noticeln(
				ruler,
				"repo:", strings.ToUpper(repo.Name), "----",
				"host:", strings.ToUpper(host), "----",
				"path:", strings.ToUpper(repo.Path),
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

type bufsend struct {
	send.Base
	buffer bytes.Buffer
}

func (b *bufsend) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString(m.String())))
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString("\n")))
	}
}
