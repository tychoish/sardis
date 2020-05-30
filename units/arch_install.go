package units

import (
	"context"
	"fmt"
	"strings"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/tychoish/sardis"
)

type archInstallPackagesJob struct {
	Names    []string `bson:"names" json:"names" yaml:"names"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const archInstallPackagesJobName = "arch-install-packages"

func init() {
	registry.AddJobType(archInstallPackagesJobName, func() amboy.Job { return archInstallPackagesFactory() })
}

func archInstallPackagesFactory() *archInstallPackagesJob {
	j := &archInstallPackagesJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    archInstallPackagesJobName,
				Version: 1,
			},
		},
	}
	return j
}

func NewArchInstallPackageJob(names []string) amboy.Job {
	j := archInstallPackagesFactory()
	j.Names = names
	j.SetID(fmt.Sprintf("%s.%d.%s", archInstallPackagesJobName, job.GetNumber(), strings.Join(names, ",")))
	return j
}

func (j *archInstallPackagesJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if len(j.Names) == 0 {
		return
	}
	args := append([]string{"pacman", "--sync", "--refresh"}, j.Names...)

	env := sardis.GetEnvironment()
	j.AddError(env.Jasper().CreateCommand(ctx).ID(j.ID()).
		Priority(level.Info).Add(args).
		SetOutputSender(level.Info, grip.GetSender()).Run(ctx))
}
