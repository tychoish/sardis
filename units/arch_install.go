package units

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
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
	out, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	grip.Debug(message.Fields{
		"id":  j.ID(),
		"cmd": strings.Join(cmd, " "),
		"err": err != nil,
		"out": strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
	})
	j.AddError(err)
}
