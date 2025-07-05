package units

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

type repoSyncJob struct {
	sardis.RepoConf
	Host string

	buildID adt.Once[string]
}

const (
	remoteUpdateCmdTemplate = "git add -A && git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

func NewLocalRepoSyncJob(repo sardis.RepoConf) fun.Worker {
	return NewRepoSyncJob("LOCAL", repo)
}

func NewRepoSyncJob(host string, repo sardis.RepoConf) fun.Worker {
	j := &repoSyncJob{
		RepoConf: repo,
		Host:     host,
	}

	j.buildID.Set(func() string {
		hostname := util.GetHostname()

		if j.isLocal() {
			return fmt.Sprintf("sync.LOCAL(%s).REPO(%s)", hostname, j.Name)
		}

		return fmt.Sprintf("sync.REMOTE(%s).SOURCE(%s).REPO(%s)", j.Host, hostname, j.Name)
	})

	return j.Run
}

func (j *repoSyncJob) isLocal() bool {
	return j.Host == "" || j.Host == "LOCAL"
}

const ruler string = "----------------"

func (j *repoSyncJob) Run(ctx context.Context) error {
	if stat, err := os.Stat(j.Path); os.IsNotExist(err) || !stat.IsDir() {
		return fmt.Errorf("path '%s' for %q does not exist", j.Path, j.buildID.Resolve())
	}
	started := time.Now()

	procout := &bufsend{}
	procout.SetPriority(level.Info)
	procout.SetName(j.buildID.Resolve())
	procout.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))
	proclog := grip.NewLogger(procout)
	proclog.Noticeln(
		ruler,
		"repo", strings.ToUpper(j.RepoConf.Name), "---",
		"host", strings.ToUpper(j.Host), "---",
		"path", strings.ToUpper(j.RepoConf.Path),
	)

	grip.Info(message.BuildPair().
		Pair("op", "repo-sync").
		Pair("state", "started").
		Pair("id", j.buildID.Resolve()).
		Pair("path", j.Path).
		Pair("host", j.Host),
	)

	conf := sardis.AppConfiguration(ctx)

	err := jasper.Context(ctx).
		CreateCommand(ctx).
		SetOutputSender(level.Info, procout).
		SetErrorSender(level.Error, procout).
		Priority(level.Debug).
		ID(j.buildID.Resolve()).
		AddEnv(sardis.EnvVarSSHAgentSocket, conf.SSHAgentSocket()).
		Directory(j.Path).
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && %s", j.Path, fmt.Sprintf(syncCmdTemplate, j.buildID.Resolve()))).
		Append(j.Pre...).
		AppendArgs("git", "add", "-A").
		Bash("git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
		Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
		AppendArgs("git", "add", "-A").
		Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", j.buildID.Resolve())).
		AppendArgs("git", "push").
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && %s", j.Path, fmt.Sprintf(syncCmdTemplate, j.buildID.Resolve()))).
		BashWhen(!j.isLocal(), "git fetch origin && git rebase origin/$(git rev-parse --abbrev-ref HEAD)").
		AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
		Append(j.Post...).
		Run(ctx)

	msg := message.BuildPair().
		Pair("op", "repo-sync").
		Pair("state", "completed").
		Pair("errors", err != nil).
		Pair("id", j.buildID.Resolve()).
		Pair("path", j.Path).
		Pair("host", j.Host).
		Pair("dur", time.Since(started).Seconds())

	if err != nil {
		proclog.Noticeln(
			ruler,
			"repo:", strings.ToUpper(j.RepoConf.Name), "----",
			"host:", strings.ToUpper(j.Host), "----",
			"path:", strings.ToUpper(j.RepoConf.Path),
			ruler,
		)

		grip.Error(msg)

		grip.Error(procout.buffer.String())

		return err
	}

	grip.Info(msg)
	return nil
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
