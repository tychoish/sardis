package units

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/tychoish/sardis"
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
		if j.Conf.ShouldSync {
			job := NewLocalRepoSyncJob(j.Conf.Path)
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
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = filepath.Dir(j.Conf.Path)

	out, err := cmd.CombinedOutput()
	grip.Debug(message.Fields{
		"id":   j.ID(),
		"cmd":  strings.Join(args, " "),
		"err":  err != nil,
		"path": j.Conf.Path,
		"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
	})
	j.AddError(err)
}