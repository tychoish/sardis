package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

type repoCloneJob struct {
	Conf     *sardis.RepoConf `bson:"conf" json:"conf" yaml:"conf"`
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

func NewRepoCloneJob(conf *sardis.RepoConf) amboy.Job {
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

	env := sardis.GetEnvironment()
	conf := env.Configuration()
	cmd := env.Jasper().CreateCommand(ctx)

	sender := send.NewAnnotatingSender(grip.GetSender(), map[string]interface{}{
		"job":  j.ID(),
		"repo": j.Conf.Name,
	})

	j.AddError(cmd.ID(j.ID()).
		Priority(level.Info).
		AddEnv(sardis.SSHAgentSocketEnvVar, conf.Settings.SSHAgentSocketPath).
		Directory(filepath.Dir(j.Conf.Path)).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Warning, sender).
		AppendArgs("git", "clone", j.Conf.Remote, j.Conf.Path).
		AddWhen(len(j.Conf.Post) > 0, j.Conf.Post).
		Run(ctx))

	grip.Notice(message.Fields{
		"path":   j.Conf.Path,
		"repo":   j.Conf.Remote,
		"errors": j.HasErrors(),
		"op":     "repo clone",
	})
}
