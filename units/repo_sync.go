package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/dependency"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

type repoSyncJob struct {
	Host     string   `bson:"host" json:"host" yaml:"host"`
	Path     string   `bson:"path" json:"path" yaml:"path"`
	PostHook []string `bson:"post" json:"post" yaml:"post"`
	PreHook  []string `bson:"pre" json:"pre" yaml:"pre"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const (
	repoSyncJobName = "repo-sync"

	remoteUpdateCmdTemplate = "git add -A && git pull --rebase --autostash --keep origin master"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

func init() { registry.AddJobType(repoSyncJobName, func() amboy.Job { return repoSyncFactory() }) }

func repoSyncFactory() *repoSyncJob {
	j := &repoSyncJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoSyncJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewLocalRepoSyncJob(path string, pre, post []string) amboy.Job {
	j := repoSyncFactory()
	j.Host = "LOCAL"
	j.Path = path
	j.PreHook = pre
	j.PostHook = post
	j.SetID(j.buildID())
	return j
}

func NewRepoSyncJob(host, path string, pre, post []string) amboy.Job {
	j := repoSyncFactory()

	j.Host = host
	j.Path = path
	j.PreHook = pre
	j.PostHook = post
	j.SetID(j.buildID())
	return j
}

func (j *repoSyncJob) buildID() string {
	tstr := time.Now().Format("2006-01-02::15.04.05")

	if j.isLocal() {
		return fmt.Sprintf("LOCAL.%s.%s.%d.%s.%s", repoSyncJobName, util.GetHostname(), job.GetNumber(), j.Path, tstr)
	}

	host, _ := os.Hostname()

	return fmt.Sprintf("REMOTE.%s.%d.%s-%s.%s.%s", repoSyncJobName, job.GetNumber(), host, j.Host, j.Path, tstr)
}

func (j *repoSyncJob) isLocal() bool {
	return j.Host == "" || j.Host == "LOCAL"
}

func (j *repoSyncJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if stat, err := os.Stat(j.Path); os.IsNotExist(err) || !stat.IsDir() {
		j.AddError(errors.Errorf("path '%s' does not exist", j.Path))
		return
	}

	grip.Info(message.Fields{
		"id": j.ID(),
		"op": "running",
	})

	env := sardis.GetEnvironment()
	conf := env.Configuration()
	cmd := env.Jasper().CreateCommand(ctx)

	sender := send.NewAnnotatingSender(grip.GetSender(), map[string]interface{}{
		"job":  j.ID(),
		"host": j.Host,
	})

	err := cmd.Priority(level.Debug).
		AddEnv(sardis.SSHAgentSocketEnvVar, conf.Settings.SSHAgentSocketPath).
		ID(j.ID()).Directory(j.Path).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Warning, sender).
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && ", j.Path)+fmt.Sprintf(syncCmdTemplate, j.ID())).
		Append(j.PreHook...).
		AppendArgs("git", "add", "-A").
		AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Bash("git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- ").
		AppendArgs("git", "add", "-A").
		Bash(fmt.Sprintf("git commit -a -m 'update: (%s)' || true", j.ID())).
		AppendArgs("git", "push").
		AppendArgsWhen(!j.isLocal(), "ssh", j.Host, fmt.Sprintf("cd %s && %s", j.Path, fmt.Sprintf(syncCmdTemplate, j.ID()))).
		AppendArgsWhen(!j.isLocal(), "git", "pull", "--keep", "--rebase", "--autostash", "origin").
		Append(j.PostHook...).Run(ctx)

	j.AddError(err)

	grip.Info(message.Fields{
		"op":     "completed repo sync",
		"errors": j.HasErrors(),
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.ID(),
	})
}
