package units

import (
	"context"
	"fmt"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/tychoish/sardis"
)

type mailSyncJob struct {
	Conf     sardis.MailConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const mailSyncJobName = "mail-sync"

func init() { registry.AddJobType(mailSyncJobName, func() amboy.Job { return repoSyncFactory() }) }

func mailSyncFactory() *mailSyncJob {
	j := &mailSyncJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    mailSyncJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewMailSyncJob(conf sardis.MailConf) amboy.Job {
	j := mailSyncFactory()
	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", mailSyncJobName, job.GetNumber(), conf.Path))
	return j
}

func (j *mailSyncJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	jobs := []amboy.Job{
		NewRepoSyncJob(j.Conf.Remote, j.Conf.Path),
	}

	if j.Conf.MuPath == "" || j.Conf.Emacs == "" {
		grip.Info(message.Fields{
			"message": "skipping mail update for repo",
			"path":    j.Conf.Path,
		})
	} else {
		jobs = append(jobs, NewMailUpdaterJob(j.Conf.Path, j.Conf.MuPath, j.Conf.Emacs, false))
	}

	for _, job := range jobs {
		grip.Info(message.Fields{
			"id": job.ID(),
			"op": "running",
		})
		job.Run(ctx)
		j.AddError(job.Error())
		if j.HasErrors() {
			break
		}
	}

}
