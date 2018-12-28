package units

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

const (
	repoSyncRemoteJobName = "repo-sync-remote"
	syncCmdTemplate       = "cd %s && git add -A && git pull --rebase --autostash --keep origin master && git commit -a -m 'auto-update: (%s)'; git push"
)

type repoSyncRemoteJob struct {
	Host     string `bson:"host" json:"host" yaml:"host"`
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

func remoteRepoSyncFactory() *repoSyncRemoteJob {
	j := &repoSyncRemoteJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoSyncRemoteJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoSyncRemoteJob(host, path string) amboy.Job {
	j := remoteRepoSyncFactory()
	j.Host = host
	j.Path = path
	j.SetID(fmt.Sprintf("SYNC.%s.%d.%s.%s.%s", repoSyncRemoteJobName, job.GetNumber(), j.Host, j.Path,
		time.Now().Format("2006-01-02::15.04.05")))
	return j
}

func (j *repoSyncRemoteJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	args := []string{"ssh", j.Host, fmt.Sprintf(syncCmdTemplate, j.Path, j.ID())}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	j.AddError(err)
	grip.Debug(message.Fields{
		"id":   j.ID(),
		"cmd":  strings.Join(args, " "),
		"err":  err != nil,
		"path": j.Path,
		"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
	})

}
