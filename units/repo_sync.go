package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
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
	repoSyncJobName = "repo-sync"

	remoteUpdateCmdTemplate = "git add -A && git pull --rebase --autostash --keep origin $(git rev-parse --abbrev-ref HEAD)"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

func NewLocalRepoSyncJob(path, branch string, pre, post []string) fun.WorkerFunc {
	j := &repoSyncJob{}
	j.Host = "LOCAL"
	j.Path = path
	j.Branch = branch
	j.PreHook = pre
	j.PostHook = post
	return j.Run
}

func NewRepoSyncJob(host, path, branch string, pre, post []string) fun.WorkerFunc {
	j := &repoSyncJob{}
	j.Host = host
	j.Path = path
	j.Branch = branch
	j.PreHook = pre
	j.PostHook = post
	return j.Run
}

func (j *repoSyncJob) buildID() string {
	return j.once.Do(func() string {
		tstr := time.Now().Format("2006-01-02::15.04.05")

		if j.isLocal() {
			return fmt.Sprintf("LOCAL.%s.%s.%s.%s", repoSyncJobName, util.GetHostname(), j.Path, tstr)
		}

		host, _ := os.Hostname()

		return fmt.Sprintf("REMOTE.%s.%s-%s.%s.%s", repoSyncJobName, host, j.Host, j.Path, tstr)
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
