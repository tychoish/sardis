package units

import (
	"context"
	"fmt"
	"os"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

type repoSyncJob struct {
	sardis.RepoConf
	Host string

	once adt.Once[string]
}

const (
	remoteUpdateCmdTemplate = "git add -A && git pull --rebase --autostash --keep origin $(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

func NewLocalRepoSyncJob(repo sardis.RepoConf) fun.Worker {
	j := &repoSyncJob{RepoConf: repo}
	j.Host = "LOCAL"
	return j.Run
}

func NewRepoSyncJob(host string, repo sardis.RepoConf) fun.Worker {
	j := &repoSyncJob{RepoConf: repo}
	j.Host = host
	return j.Run
}

func (j *repoSyncJob) buildID() string {
	j.once.Do(func() string {
		hostname := util.GetHostname()

		if j.isLocal() {
			return fmt.Sprintf("LOCAL(%s).sync=[%s]", hostname, j.Name)
		}

		return fmt.Sprintf("REMOTE(%s).sync.(FROM(%s))=[%s]", j.Host, hostname, j.Name)
	})
	return j.once.Resolve()
}

func (j *repoSyncJob) isLocal() bool {
	return j.Host == "" || j.Host == "LOCAL"
}

func (j *repoSyncJob) Run(ctx context.Context) error {
	if stat, err := os.Stat(j.Path); os.IsNotExist(err) || !stat.IsDir() {
		return fmt.Errorf("path '%s' for %q does not exist", j.Path, j.buildID())
	}

	grip.Info(message.BuildPair().
		Pair("op", "repo-sync").
		Pair("state", "started").
		Pair("id", j.buildID()).
		Pair("path", j.Path).
		Pair("host", j.Host),
	)

	conf := sardis.AppConfiguration(ctx)

	err := jasper.Context(ctx).
		CreateCommand(ctx).
		Priority(level.Debug).
		ID(j.buildID()).
		AddEnv(sardis.SSHAgentSocketEnvVar, conf.SSHAgentSocket()).
		Directory(j.Path).
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && ", j.Path)+fmt.Sprintf(syncCmdTemplate, j.buildID())).
		Append(j.Pre...).
		AppendArgs("git", "add", "-A").
		AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
		AppendArgs("git", "add", "-A").
		Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", j.buildID())).
		AppendArgs("git", "push").
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && %s", j.Path, fmt.Sprintf(syncCmdTemplate, j.buildID()))).
		AppendArgsWhen(!j.isLocal(), "git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Append(j.Post...).
		Run(ctx)

	grip.Info(message.BuildPair().
		Pair("op", "repo-sync").
		Pair("state", "completed").
		Pair("errors", err != nil).
		Pair("id", j.buildID()).
		Pair("path", j.Path).
		Pair("host", j.Host),
	)
	return err
}
