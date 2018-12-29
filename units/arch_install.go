package units

import (
	"context"
	"fmt"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip/level"
	"github.com/tychoish/sardis/util"
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

	cmd := []string{"pacman", "--sync", "--refresh"}
	cmd = append(cmd, j.Names...)

	j.AddError(util.RunCommand(ctx, j.ID(), level.Debug, cmd, "", nil))
}
