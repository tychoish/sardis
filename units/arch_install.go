package units

import (
	"context"
	"fmt"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
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
	j.AddError(env.Jasper().CreateCommand(ctx).Add(args).
		SetOutputSender(level.Debug, grip.GetSender()).ID(j.ID()).Run(ctx))
}
