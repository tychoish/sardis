package units

import (
	"context"
	"fmt"
	"os"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
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

	jpm := sardis.GetEnvironment().Jasper()

	j.AddError(jpm.CreateCommand(ctx).ID(j.ID()).SetOutputSender(level.Debug, grip.GetSender()).
		Append("git", "pull", "--keep", "--rebase", "--autostash", j.Conf.RemoteName, j.Conf.Branch).
		Directory(j.Conf.Path).Run(ctx))

	if j.HasErrors() {
		return
	}

	if j.Conf.Post == nil {
		return
	}

	j.AddError(jpm.CreateCommand(ctx).ID(j.ID()).SetOutputSender(level.Info, grip.GetSender()).
		Directory(j.Conf.Path).Add(j.Conf.Post).Run(ctx))
}
