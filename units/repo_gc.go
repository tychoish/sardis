package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/pkg/errors"
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
		j.AddError(errors.Errorf("cannot cleanup %s, no repository exists", j.Path))
		return
	}

	grip.Info(message.Fields{
		"id": j.ID(),
		"op": "running",
	})

	cmd := sardis.GetEnvironment().Jasper()

	startAt := time.Now()

	j.AddError(cmd.CreateCommand(ctx).Priority(level.Info).
		ID(j.ID()).Directory(j.Path).
		SetCombinedSender(level.Info, grip.GetSender()).
		AppendArgs("git", "gc").
		Run(ctx))

	grip.Notice(message.Fields{
		"op":     "git gc",
		"repo":   j.Path,
		"secs":   time.Since(startAt).Seconds(),
		"errors": j.HasErrors(),
	})
}
