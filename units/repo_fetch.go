package units

import (
	"context"
	"fmt"
	"os"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/tychoish/sardis"
)

type repoFetchJob struct {
	Conf     sardis.RepoConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoFetchJobName = "repo-fetch"

func init() { registry.AddJobType(repoFetchJobName, func() amboy.Job { return repoFetchFactory() }) }

func repoFetchFactory() *repoFetchJob {
	j := &repoFetchJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoFetchJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoFetchJob(conf sardis.RepoConf) amboy.Job {
	j := repoFetchFactory()

	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", repoFetchJobName, job.GetNumber(), j.Conf.Path))
	return j
}

func (j *repoFetchJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if _, err := os.Stat(j.Conf.Path); os.IsNotExist(err) {
		grip.Info(message.Fields{
			"id":   j.ID(),
			"path": j.Conf.Path,
			"op":   "repo doesn't exist; cloning",
		})

		job := NewRepoCloneJob(j.Conf)
		job.Run(ctx)
		j.AddError(job.Error())
		return
	}

	cmd := sardis.GetEnvironment().Jasper().CreateCommand(ctx)

	j.AddError(cmd.ID(j.ID()).Directory(j.Conf.Path).
		SetOutputSender(level.Info, grip.GetSender()).
		Append("git", "pull", "--keep", "--rebase", "--autostash", j.Conf.RemoteName, j.Conf.Branch).
		Add(j.Conf.Post).
		Run(ctx))

	grip.Notice(message.Fields{
		"path":   j.Conf.Path,
		"repo":   j.Conf.Remote,
		"errors": j.HasErrors(),
		"op":     "repo fetch",
	})
}
