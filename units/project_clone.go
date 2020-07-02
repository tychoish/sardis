package units

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/deciduosity/utility"
	"github.com/tychoish/sardis"
)

type projectCloneJob struct {
	Conf     sardis.ProjectConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const projectCloneJobName = "project-clone"

func init() {
	registry.AddJobType(projectCloneJobName, func() amboy.Job { return projectCloneJobFactory() })
}

func projectCloneJobFactory() *projectCloneJob {
	j := &projectCloneJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    projectCloneJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewProjectCloneJob(conf sardis.ProjectConf) amboy.Job {
	j := projectCloneJobFactory()
	j.SetID(fmt.Sprintf("%s.%d.%s", projectCloneJobName, job.GetNumber(), conf.Name))
	j.Conf = conf
	return j
}

func (j *projectCloneJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	jpm := sardis.GetEnvironment().Jasper()

	for _, r := range j.Conf.Repositories {
		path := filepath.Join(j.Conf.Options.Directory, r.Name)
		if utility.FileExists(path) {
			grip.Warning(message.Fields{
				"project": j.Conf.Name,
				"path":    path,
				"name":    r.Name,
				"op":      "checkout already exists",
			})
			continue
		}

		cmd := jpm.CreateCommand(ctx).ID(fmt.Sprintf("%s.%s", j.ID(), r.Name)).
			Directory(j.Conf.Options.Directory).
			SetErrorSender(level.Error, grip.GetSender()).
			SetOutputSender(level.Info, grip.GetSender()).
			AppendArgs("git", "clone", fmt.Sprintf("git@github.com:%s/%s.git", j.Conf.Options.GithubOrg, r.Name))

		grip.Notice(message.Fields{
			"project": j.Conf.Name,
			"path":    path,
			"name":    r.Name,
			"op":      "clone",
		})

		j.AddError(cmd.Run(ctx))
	}
}
