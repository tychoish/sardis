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
	"github.com/mongodb/grip/message"
	"github.com/tychoish/sardis"
)

type symlinkCreateJob struct {
	Conf     sardis.LinkConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const symlinkCreateJobName = "symlink-create"

func init() {
	registry.AddJobType(symlinkCreateJobName, func() amboy.Job { return symlinkCreateFactory() })
}

func symlinkCreateFactory() *symlinkCreateJob {
	j := &symlinkCreateJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    symlinkCreateJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewSymlinkCreateJob(conf sardis.LinkConf) amboy.Job {
	j := symlinkCreateFactory()

	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", symlinkCreateJobName, job.GetNumber(), j.Conf.Target))
	return j
}

func (j *symlinkCreateJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	dst := filepath.Join(j.Conf.Path, j.Conf.Name)

	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		if !j.Conf.Update {
			return
		}
		target, err := filepath.EvalSymlinks(dst)
		if err != nil {
			j.AddError(err)
			return
		}

		if target != j.Conf.Target {
			j.AddError(os.Remove(dst))
			grip.Info(message.Fields{
				"id":         j.ID(),
				"op":         "removed incorrect link target",
				"old_target": target,
				"new_target": j.Conf.Target,
				"link":       dst,
				"err":        j.HasErrors(),
			})
		}
	}

	j.AddError(os.Symlink(j.Conf.Target, dst))

	grip.Info(message.Fields{
		"op":  "created new symbolic link",
		"id":  j.ID(),
		"src": j.Conf.Target,
		"dst": dst,
		"err": j.HasErrors(),
	})
}
