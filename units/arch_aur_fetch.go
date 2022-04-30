package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
)

type archAurFetchJob struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Update   bool   `bson:"update" json:"update" yaml:"update"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const archAurFetchJobName = "arch-aur-fetch"

func init() {
	registry.AddJobType(archAurFetchJobName, func() amboy.Job { return archAurFetchFactory() })
}

func archAurFetchFactory() *archAurFetchJob {
	j := &archAurFetchJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    archAurFetchJobName,
				Version: 1,
			},
		},
	}
	return j
}

func NewArchFetchAurJob(name string, update bool) amboy.Job {
	j := archAurFetchFactory()
	j.Name = name
	j.Update = update
	j.SetID(fmt.Sprintf("%s.%d.%s", archAurFetchJobName, job.GetNumber(), name))
	return j
}

func (j *archAurFetchJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.Name == "" {
		j.AddError(errors.New("name is not specified"))
		return
	}

	env := sardis.GetEnvironment()
	conf := env.Configuration()
	dir := filepath.Join(conf.System.Arch.BuildPath, j.Name)
	args := []string{}

	if stat, err := os.Stat(dir); os.IsNotExist(err) {
		args = append(args, "git", "clone", fmt.Sprintf("https://aur.archlinux.org/%s.git", j.Name))
		dir = filepath.Dir(dir)
	} else if !stat.IsDir() {
		j.AddError(errors.Errorf("%s exists and is not a directory", dir))
		return
	} else if j.Update {
		args = append(args, "git", "pull", "origin", "master")
	} else {
		grip.Infof("fetch package for '%s' is a noop", j.Name)
		return
	}

	j.AddError(env.Jasper().CreateCommand(ctx).ID(j.ID()).Directory(dir).
		Priority(level.Info).SetOutputSender(level.Debug, grip.Sender()).
		AppendArgs(args...).Run(ctx))
}
