package units

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
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
	dir := filepath.Join(conf.Arch.BuildPath, j.Name)
	pkgbuild := filepath.Join(dir, "PKGBUILD")

	if _, err := os.Stat(pkgbuild); os.IsNotExist(err) {
		j.AddError(errors.Errorf("%s does not exist", pkgbuild))
		return
	}

	args := []string{"makepkg", "--syncdeps", "--force", "--install"}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	grip.Info(message.Fields{
		"id":   j.ID(),
		"cmd":  strings.Join(args, " "),
		"err":  err != nil,
		"path": dir,
		"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
	})
	j.AddError(err)
}
