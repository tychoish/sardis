package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/dependency"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/sardis"
)

type repoCleanupJob struct {
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoCleanupJobName = "repo-gc"

func init() {
	registry.AddJobType(repoCleanupJobName, func() amboy.Job { return repoCleanupJobFactory() })
}

func repoCleanupJobFactory() *repoCleanupJob {
	j := &repoCleanupJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoCleanupJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoCleanupJob(path string) amboy.Job {
	j := repoCleanupJobFactory()
	j.Path = path
	j.SetID(fmt.Sprintf("%s.%s.%d", repoCleanupJobName, path, job.GetNumber()))
	return j
}

func (j *repoCleanupJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if _, err := os.Stat(j.Path); os.IsNotExist(err) {
		j.AddError(fmt.Errorf("cannot cleanup %s, no repository exists", j.Path))
		return
	}

	grip.Info(message.Fields{
		"id": j.ID(),
		"op": "running",
	})

	cmd := sardis.GetEnvironment().Jasper()

	startAt := time.Now()
	sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		"job":  j.ID(),
		"path": j.Path,
	})

	j.AddError(cmd.CreateCommand(ctx).Priority(level.Debug).
		ID(j.ID()).Directory(j.Path).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Warning, sender).
		AppendArgs("git", "gc").
		AppendArgs("git", "prune").
		Run(ctx))

	grip.Notice(message.Fields{
		"op":     "git gc",
		"repo":   j.Path,
		"secs":   time.Since(startAt).Seconds(),
		"errors": j.HasErrors(),
	})
}
