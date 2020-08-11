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
	"github.com/deciduosity/utility"
	"github.com/tychoish/sardis"
)

type projectFetchJob struct {
	Conf     sardis.ProjectConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const projectFetchJobName = "project-fetch"

func init() {
	registry.AddJobType(projectFetchJobName, func() amboy.Job { return projectFetchJobFactory() })
}

func projectFetchJobFactory() *projectFetchJob {
	j := &projectFetchJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    projectFetchJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewProjectFetchJob(conf sardis.ProjectConf) amboy.Job {
	j := projectFetchJobFactory()
	j.SetID(fmt.Sprintf("%s.%d.%s", projectFetchJobName, job.GetNumber(), conf.Name))
	j.Conf = conf
	return j
}

func (j *projectFetchJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	jpm := sardis.GetEnvironment().Jasper()

	if _, err := os.Stat(j.Conf.Name.Options.Directory); os.IsNotExist(err) {
		if err := os.MkdirAll(j.Conf.Options.Directory, 0755); err != nil {
			j.AddError(err)
			return
		}
	}

	for _, r := range j.Conf.Repositories {
		path := filepath.Join(j.Conf.Options.Directory, r.Name)
		if !utility.FileExists(path) {
			grip.Warning(message.Fields{
				"project": j.Conf.Name,
				"path":    path,
				"name":    r.Name,
				"op":      "checkout does not exist",
			})
			continue
		}

		cmd := jpm.CreateCommand(ctx).ID(fmt.Sprintf("%s.%s", j.ID(), r.Name)).
			Directory(path).
			SetErrorSender(level.Error, grip.GetSender()).
			SetOutputSender(level.Info, grip.GetSender()).
			AppendArgs("git", "pull", "--rebase", "--autostash")

		grip.Notice(message.Fields{
			"project": j.Conf.Name,
			"path":    path,
			"name":    r.Name,
			"op":      "fetch",
		})

		j.AddError(cmd.Run(ctx))
	}
}
