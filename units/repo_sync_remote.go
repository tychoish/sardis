package units

import (
	"context"
	"fmt"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/tychoish/sardis"
)

const (
	repoSyncRemoteJobName = "repo-sync-remote"

	remoteUpdateCmdTemplate = "git add -A && git pull --rebase --autostash --keep origin master"
	syncCmdTemplate         = remoteUpdateCmdTemplate + " && git commit -a -m 'auto-update: (%s)'; git push"
)

type repoSyncRemoteJob struct {
	Host     string   `bson:"host" json:"host" yaml:"host"`
	Path     string   `bson:"path" json:"path" yaml:"path"`
	PostHook []string `bson:"post" json:"post" yaml:"post"`
	PreHook  []string `bson:"pre" json:"pre" yaml:"pre"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func remoteRepoSyncFactory() *repoSyncRemoteJob {
	j := &repoSyncRemoteJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoSyncRemoteJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoSyncRemoteJob(host, path string, pre, post []string) amboy.Job {
	j := remoteRepoSyncFactory()
	j.Host = host
	j.Path = path
	j.PreHook = pre
	j.PostHook = post
	j.SetID(fmt.Sprintf("SYNC.%s.%d.%s.%s.%s", repoSyncRemoteJobName, job.GetNumber(), j.Host, j.Path,
		time.Now().Format("2006-01-02::15.04.05")))
	return j
}

func (j *repoSyncRemoteJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	grip.Info(message.Fields{
		"id": j.ID(),
		"op": "running",
	})

	err := sardis.GetEnvironment().Jasper().CreateCommand(ctx).
		ID(j.ID()).Priority(level.Info).
		Directory(j.Path).Host(j.Host).
		SetOutputSender(level.Info, grip.GetSender()).
		Append(j.PreHook...).
		AppendArgs(fmt.Sprintf(syncCmdTemplate, j.ID())).
		Append(j.PostHook...).
		Run(ctx)

	j.AddError(err)

	grip.Info(message.Fields{
		"op":     "completed repo sync",
		"errors": j.HasErrors(),
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.ID(),
	})
}
