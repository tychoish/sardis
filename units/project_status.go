package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cdr/amboy"
	"github.com/cdr/amboy/dependency"
	"github.com/cdr/amboy/job"
	"github.com/cdr/amboy/registry"
	"github.com/cdr/grip"
	"github.com/cdr/grip/level"
	"github.com/cdr/grip/message"
	"github.com/cdr/grip/send"
	"github.com/tychoish/sardis"
)

type projectStatusJob struct {
	Conf     sardis.ProjectConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const projectStatusJobName = "project-status"

func init() {
	registry.AddJobType(projectStatusJobName, func() amboy.Job { return projectStatusJobFactory() })
}

func projectStatusJobFactory() *projectStatusJob {
	j := &projectStatusJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    projectStatusJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewProjectStatusJob(conf sardis.ProjectConf) amboy.Job {
	j := projectStatusJobFactory()
	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", projectStatusJobName, job.GetNumber(), conf.Name))
	return j
}

func (j *projectStatusJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	jpm := sardis.GetEnvironment().Jasper()

	for _, r := range j.Conf.Repositories {
		path := filepath.Join(j.Conf.Options.Directory, r.Name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			grip.Warning(message.Fields{
				"project": j.Conf.Name,
				"path":    path,
				"name":    r.Name,
				"status":  "does not exist",
			})
			continue
		}

		output := send.NewAnnotatingSender(grip.GetSender(), message.Fields{"repo": j.Conf.Name})

		cmd := jpm.CreateCommand(ctx).ID(j.ID()).Directory(path).
			SetOutputSender(level.Info, output).
			SetErrorSender(level.Error, output).
			Add(getStatusCommandArgs(path)).
			AppendArgs("git", "status", "--short", "--branch")

		grip.Notice(message.Fields{
			"project": j.Conf.Name,
			"path":    path,
			"name":    r.Name,
			"op":      "status",
		})

		j.AddError(cmd.Run(ctx))
	}
}
