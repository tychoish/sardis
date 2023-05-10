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
	Host     string   `bson:"host" json:"host" yaml:"host"`
	Path     string   `bson:"path" json:"path" yaml:"path"`
	Branch   string   `bson:"branch" json:"branch" yaml:"branch"`
	PostHook []string `bson:"post" json:"post" yaml:"post"`
	PreHook  []string `bson:"pre" json:"pre" yaml:"pre"`

	once adt.Once[string]
}

const (
	remoteUpdateCmdTemplate = "git add -A && git pull --rebase --autostash --keep origin $(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

func NewLocalRepoSyncJob(repo sardis.RepoConf) fun.WorkerFunc {
	j := &repoSyncJob{}
	j.Host = "LOCAL"
	j.Path = repo.Path
	j.Branch = repo.Branch
	j.PreHook = repo.Pre
	j.PostHook = repo.Post
	return j.Run
}

func NewRepoSyncJob(host string, repo sardis.RepoConf) fun.WorkerFunc {
	j := &repoSyncJob{}
	j.Host = host
	j.Path = repo.Path
	j.Branch = repo.Branch
	j.PreHook = repo.Pre
	j.PostHook = repo.Post
	return j.Run
}

func (j *repoSyncJob) buildID() string {
	return j.once.Do(func() string {
		hostname := util.GetHostname()

		if j.isLocal() {
			return fmt.Sprintf("LOCAL(%s).sync", hostname)
		}

		return fmt.Sprintf("REMOTE(%s).sync.FROM(%s)", j.Host, hostname)
	})
}

func (j *repoSyncJob) isLocal() bool {
	return j.Host == "" || j.Host == "LOCAL"
}

func (j *repoSyncJob) Run(ctx context.Context) error {
	if stat, err := os.Stat(j.Path); os.IsNotExist(err) || !stat.IsDir() {
		return fmt.Errorf("path '%s' does not exist", j.Path)
	}

	grip.Info(message.Fields{
		"op": "running",
		"id": j.buildID(),
	})

	conf := sardis.AppConfiguration(ctx)

	err := jasper.Context(ctx).
		CreateCommand(ctx).
		Priority(level.Info).
		ID(j.buildID()).
		AddEnv(sardis.SSHAgentSocketEnvVar, conf.SSHAgentSocket()).
		Directory(j.Path).
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && ", j.Path)+fmt.Sprintf(syncCmdTemplate, j.buildID())).
		Append(j.PreHook...).
		AppendArgs("git", "add", "-A").
		AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
		AppendArgs("git", "add", "-A").
		Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", j.buildID())).
		AppendArgs("git", "push").
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && %s", j.Path, fmt.Sprintf(syncCmdTemplate, j.buildID()))).
		AppendArgsWhen(!j.isLocal(), "git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Append(j.PostHook...).
		Run(ctx)

	grip.Info(message.Fields{
		"op":     "completed repo sync",
		"errors": err != nil,
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.buildID(),
	})
	return err
}
