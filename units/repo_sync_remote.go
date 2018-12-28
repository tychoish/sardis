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
	"github.com/tychoish/sardis"
)

const (
	repoSyncRemoteJobName = "repo-sync-remote"
	syncCmdTemplate       = "cd %s && git add -A && git pull --rebase --autostash --keep origin master && git commit -a -m 'auto-update: (%s)'; git push"
)

type repoSyncRemoteJob struct {
	Host     string   `bson:"host" json:"host" yaml:"host"`
	Path     string   `bson:"path" json:"path" yaml:"path"`
	PostHook []string `bson:"post" json:"post" yaml:"post"`
	PreHook  []string `bson:"pre" json:"pre" yaml:"pre"`
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

func NewRepoSyncRemoteJob(host, path string, pre, post []string) amboy.Job {
	j := remoteRepoSyncFactory()
	j.Host = host
	j.Path = path
	j.PreHook = pre
	j.PostHook = post
	j.SetID(fmt.Sprintf("SYNC.%s.%d.%s.%s.%s", repoSyncRemoteJobName, job.GetNumber(), j.Host, j.Path,
		time.Now().Format("2006-01-02::15.04.05")))
	return j
}

func (j *repoSyncRemoteJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	grip.Info(message.Fields{
		"id": j.ID(),
		"op": "running",
	})

	cmds := [][]string{}

	for _, cmd := range j.PreHook {
		cmds = append(cmds, []string{"ssh", j.Host, cmd})
	}

	cmds = append(cmds, []string{"ssh", j.Host, fmt.Sprintf(syncCmdTemplate, j.Path, j.ID())})

	for _, cmd := range j.PostHook {
		cmds = append(cmds, []string{"ssh", j.Host, cmd})
	}

	for idx, args := range cmds {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		j.AddError(err)
		grip.Debug(message.Fields{
			"id":   j.ID(),
			"cmd":  strings.Join(args, " "),
			"err":  err != nil,
			"path": j.Path,
			"idx":  idx,
			"num":  len(cmds),
			"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
		})
	}

	notify := sardis.GetEnvironment().Logger()
	msg := message.Fields{
		"op":     "completed repo sync",
		"errors": j.HasErrors(),
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.ID(),
	}
	notify.Notice(msg)
	grip.Info(msg)

}
