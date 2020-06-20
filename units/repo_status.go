package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/deciduosity/grip/send"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
)

type repoStatusJob struct {
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoStatusJobName = "repo-status"

func init() {
	registry.AddJobType(repoStatusJobName, func() amboy.Job { return repoStatusJobFactory() })
}

func repoStatusJobFactory() *repoStatusJob {
	j := &repoStatusJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoStatusJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoStatusJob(path string) amboy.Job {
	j := repoStatusJobFactory()
	j.Path = path
	j.SetID(fmt.Sprintf("%s.%s.%d", repoStatusJobName, path, job.GetNumber()))
	return j
}

func (j *repoStatusJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if _, err := os.Stat(j.Path); os.IsNotExist(err) {
		j.AddError(errors.Errorf("cannot check status %s, no repository exists", j.Path))
		return
	}

	grip.Info(message.Fields{
		"id":   j.ID(),
		"op":   "running",
		"path": j.Path,
	})

	cmd := sardis.GetEnvironment().Jasper()

	startAt := time.Now()

	output := send.NewAnnotatingSender(grip.GetSender(), message.Fields{"path": j.Path})
	j.AddError(cmd.CreateCommand(ctx).Priority(level.Info).
		ID(j.ID()).Directory(j.Path).
		SetOutputSender(level.Info, output).
		Add(getStatusCommandArgs(j.Path)).
		AppendArgs("git", "status", "--short", "--branch").
		Run(ctx))

	grip.Debug(message.Fields{
		"op":     "git status",
		"repo":   j.Path,
		"secs":   time.Since(startAt).Seconds(),
		"errors": j.HasErrors(),
	})
}

func getStatusCommandArgs(path string) []string {
	return []string{
		"git", "log", "--date=relative", "--decorate", "-n", "1",
		fmt.Sprintf("--format=%s:", filepath.Base(path)) + `%N (%cr) "%s"`,
	}

}
