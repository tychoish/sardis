package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/pkg/errors"
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
		j.AddError(errors.Errorf("%s does not exist", pkgbuild))
		return
	}

	j.AddError(env.Jasper().CreateCommand(ctx).
		AppendArgs("makepkg", "--syncdeps", "--force", "--install", "--noconfirm").
		SetOutputSender(level.Info, grip.GetSender()).ID(j.ID()).Directory(dir).Run(ctx))
}
