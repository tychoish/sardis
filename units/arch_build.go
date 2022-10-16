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

type archAbsBuildJob struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const archAbsBuildJobName = "arch-abs-build"

func init() {
	registry.AddJobType(archAbsBuildJobName, func() amboy.Job { return archAbsBuildFactory() })
}

func archAbsBuildFactory() *archAbsBuildJob {
	j := &archAbsBuildJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    archAbsBuildJobName,
				Version: 1,
			},
		},
	}
	return j
}

func NewArchAbsBuildJob(name string) amboy.Job {
	j := archAbsBuildFactory()
	j.Name = name
	j.SetID(fmt.Sprintf("%s.%d.%s", archAbsBuildJobName, job.GetNumber(), name))
	return j
}

func (j *archAbsBuildJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if j.Name == "" {
		j.AddError(errors.New("name is not specified"))
		return
	}

	env := sardis.GetEnvironment()
	conf := env.Configuration()
	dir := filepath.Join(conf.System.Arch.BuildPath, j.Name)
	pkgbuild := filepath.Join(dir, "PKGBUILD")

	if _, err := os.Stat(pkgbuild); os.IsNotExist(err) {
		j.AddError(fmt.Errorf("%s does not exist", pkgbuild))
		return
	}

	j.AddError(env.Jasper().CreateCommand(ctx).ID(j.ID()).Priority(level.Info).
		AppendArgs("makepkg", "--syncdeps", "--force", "--install", "--noconfirm").
		SetOutputSender(level.Info, grip.Sender()).ID(j.ID()).Directory(dir).Run(ctx))
}
