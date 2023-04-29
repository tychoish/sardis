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
	"github.com/tychoish/jasper"
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

	if _, err := os.Stat(j.Conf.Target); os.IsNotExist(err) {
		grip.Notice(message.Fields{
			"message": "missing target",
			"name":    j.Conf.Name,
			"target":  j.Conf.Target,
			"id":      j.ID(),
		})
		return
	}

	jpm := jasper.Context(ctx)

	if _, err := os.Stat(j.Conf.Path); !os.IsNotExist(err) {
		if !j.Conf.Update {
			return
		}

		target, err := filepath.EvalSymlinks(j.Conf.Path)
		if err != nil {
			j.AddError(err)
			return
		}

		if target != j.Conf.Target {
			if j.Conf.RequireSudo {
				j.AddError(jpm.CreateCommand(ctx).Sudo(true).
					SetCombinedSender(level.Info, grip.Sender()).
					AppendArgs("rm", dst).Run(ctx))
			} else {
				j.AddError(os.Remove(dst))
			}

			grip.Info(message.Fields{
				"id":         j.ID(),
				"op":         "removed incorrect link target",
				"old_target": target,
				"name":       j.Conf.Name,
				"target":     j.Conf.Target,
				"err":        j.HasErrors(),
			})
		} else {
			return
		}

	}

	linkDir := filepath.Dir(j.Conf.Target)
	if j.Conf.RequireSudo {
		cmd := jpm.CreateCommand(ctx).Sudo(true).
			SetCombinedSender(level.Info, grip.Sender())

		if _, err := os.Stat(linkDir); os.IsNotExist(err) {
			cmd.AppendArgs("mkdir", "-p", linkDir)
		}

		cmd.AppendArgs("ln", "-s", j.Conf.Target, j.Conf.Path)

		j.AddError(cmd.Run(ctx))

	} else {
		if _, err := os.Stat(linkDir); os.IsNotExist(err) {
			j.AddError(os.MkdirAll(linkDir, 0755))
		}

		j.AddError(os.Symlink(j.Conf.Target, j.Conf.Path))
	}

	grip.Info(message.Fields{
		"op":  "created new symbolic link",
		"id":  j.ID(),
		"src": j.Conf.Path,
		"dst": j.Conf.Target,
		"err": j.HasErrors(),
	})
}
