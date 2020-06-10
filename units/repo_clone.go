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

type repoCloneJob struct {
	Conf     sardis.RepoConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoCloneJobName = "repo-clone"

func init() { registry.AddJobType(repoCloneJobName, func() amboy.Job { return repoCloneFactory() }) }

func repoCloneFactory() *repoCloneJob {
	j := &repoCloneJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoCloneJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoCloneJob(conf sardis.RepoConf) amboy.Job {
	j := repoCloneFactory()

	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", repoCloneJobName, job.GetNumber(), j.Conf.Path))
	return j
}

func (j *repoCloneJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if _, err := os.Stat(j.Conf.Path); !os.IsNotExist(err) {
		if j.Conf.LocalSync {
			job := NewLocalRepoSyncJob(j.Conf.Path, nil, nil)
			job.Run(ctx)
			j.AddError(job.Error())
		} else if j.Conf.Fetch {
			job := NewRepoFetchJob(j.Conf)
			job.Run(ctx)
			j.AddError(job.Error())
		}

		grip.Notice(message.Fields{
			"path": j.Conf.Path,
			"repo": j.Conf.Remote,
			"op":   "exists, skipping clone",
		})

		return
	}

	cmd := sardis.GetEnvironment().Jasper().CreateCommand(ctx)

	j.AddError(cmd.ID(j.ID()).
		Priority(level.Info).
		Directory(j.Conf.Path).
		SetOutputSender(level.Info, grip.GetSender()).
		AppendArgs("git", "clone", j.Conf.Remote, j.Conf.Path).
		Add(j.Conf.Post).
		Run(ctx))

	grip.Notice(message.Fields{
		"path":   j.Conf.Path,
		"repo":   j.Conf.Remote,
		"errors": j.HasErrors(),
		"op":     "repo clone",
	})
}
