package units

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

type repoSyncJob struct {
	Host     string `bson:"host" json:"host" yaml:"host"`
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoSyncJobName = "repo-sync"

func init() { registry.AddJobType(repoSyncJobName, func() amboy.Job { return repoSyncFactory() }) }

func repoSyncFactory() *repoSyncJob {
	j := &repoSyncJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoSyncJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewLocalRepoSyncJob(path string) amboy.Job {
	j := repoSyncFactory()
	j.Host = "LOCAL"
	j.Path = path
	j.SetID(j.buildID())
	return j
}

func NewRepoSyncJob(host, path string) amboy.Job {
	j := repoSyncFactory()

	j.Host = host
	j.Path = path
	j.SetID(j.buildID())
	return j
}

func (j *repoSyncJob) buildID() string {
	tstr := time.Now().Format("2006-01-02::15.04.05")

	if j.isLocal() {
		return fmt.Sprintf("LOCAL.%s.%d.%s.%s", repoSyncJobName, job.GetNumber(), j.Path, tstr)
	}

	return fmt.Sprintf("REMOTE.%s.%d.%s.%s.%s", repoSyncJobName, job.GetNumber(), j.Host, j.Path, tstr)
}

func (j *repoSyncJob) isLocal() bool {
	return j.Host == "" || j.Host == "LOCAL"
}

func (j *repoSyncJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if stat, err := os.Stat(j.Path); os.IsNotExist(err) || !stat.IsDir() {
		j.AddError(errors.Errorf("path '%s' does not exist", j.Path))
	}

	cmds := [][]string{}

	if !j.isLocal() {
		cmds = append(cmds,
			[]string{"ssh", j.Host,
				fmt.Sprintf("cd %s && git add . && git pull && git commit -a -m 'mail update (%s)'; git push", j.Path, j.ID()),
			})
	}

	cmds = append(cmds,
		[]string{"git", "pull", "--keep", "--rebase", "--autostash", "origin", "master"},
		[]string{"bash", "-c", "git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- "},
		[]string{"git", "add", "-A"},
		[]string{"git", "commit", "-a", "-m", fmt.Sprintf("mail update %s", j.ID())},
		[]string{"git", "push"},
	)

	if !j.isLocal() {
		cmds = append(cmds,
			[]string{"ssh", j.Host,
				fmt.Sprintf("cd %s && git add . && git pull && git commit -a -m 'mail update (%s)'; git push", j.Path, j.ID()),
			},
			[]string{"git", "pull", "--keep", "--rebase", "--autostash", "origin", "master"},
		)
	}

	for idx, cmd := range cmds {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = j.Path

		out, err := c.CombinedOutput()
		grip.Debug(message.Fields{
			"id":   j.ID(),
			"cmd":  strings.Join(cmd, " "),
			"err":  err != nil,
			"path": j.Path,
			"idx":  idx,
			"num":  len(cmds),
			"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
		})

		if err != nil {
			if cmd[0] == "git" {
				grip.Debug("skipping git error")
				continue
			}
			j.AddError(err)
			break
		}
	}
	grip.Info(message.Fields{
		"op":     "completed repo sync",
		"errors": j.HasErrors(),
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.ID(),
	})
}
