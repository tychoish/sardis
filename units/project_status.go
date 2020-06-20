package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
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
		cmd := jpm.CreateCommand(ctx).ID(j.ID()).Directory(j.Conf.Options.Directory).
			SetOutputSender(level.Info, grip.GetSender()).
			Add(getStatusCommandArgs(path))

		grip.Notice(message.Fields{
			"project": j.Conf.Name,
			"path":    path,
			"name":    r.Name,
		})
		j.AddError(cmd.Run(ctx))
	}

}
