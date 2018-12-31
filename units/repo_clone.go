package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
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
			job := NewLocalRepoSyncJob(j.Conf.Path)
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
	args := []string{"git", "clone", j.Conf.Remote, j.Conf.Path}

	err := util.RunCommand(ctx, j.ID(), level.Debug, args, filepath.Dir(j.Conf.Path), nil)
	if err != nil {
		j.AddError(err)
		return
	}

	if j.Conf.Post == nil {
		return
	}

	j.AddError(util.NewCommand().ID(j.ID()).Priority(level.Info).Directory(j.Conf.Path).Append(j.Conf.Post...).Run(ctx))

}
