package units

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
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

	jobs := []amboy.Job{}
	hostname, _ := os.Hostname()

	for _, m := range j.Conf.Mirrors {
		if strings.Contains(m, hostname) {
			continue
		}
		jobs = append(jobs, NewRepoSyncRemoteJob(m, j.Conf.Path, nil, nil))
	}

	jobs = append(jobs, NewRepoSyncJob(j.Conf.Remote, j.Conf.Path, nil, nil))

	if j.Conf.MuPath == "" || j.Conf.Emacs == "" {
		grip.Debug(message.Fields{
			"message": "skipping mail update for repo",
			"path":    j.Conf.Path,
			"name":    j.Conf.Name,
		})
	} else {
		jobs = append(jobs, NewMailUpdaterJob(j.Conf.Path, j.Conf.MuPath, j.Conf.Emacs, false))
	}

	for _, job := range jobs {
		job.Run(ctx)
		j.AddError(job.Error())
		if j.HasErrors() {
			break
		}
	}
}
