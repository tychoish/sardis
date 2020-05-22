package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/shlex"
	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/level"
	"github.com/deciduosity/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

type repoSyncJob struct {
	Host     string   `bson:"host" json:"host" yaml:"host"`
	Path     string   `bson:"path" json:"path" yaml:"path"`
	PostHook []string `bson:"post" json:"post" yaml:"post"`
	PreHook  []string `bson:"pre" json:"pre" yaml:"pre"`
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

func NewRepoSyncJob(host, path string, pre, post []string) amboy.Job {
	j := repoSyncFactory()

	j.Host = host
	j.Path = path
	j.PreHook = pre
	j.PostHook = post
	j.SetID(j.buildID())
	return j
}

func (j *repoSyncJob) buildID() string {
	tstr := time.Now().Format("2006-01-02::15.04.05")

	if j.isLocal() {
		return fmt.Sprintf("LOCAL.%s.%s.%d.%s.%s", repoSyncJobName, util.GetHostname(), job.GetNumber(), j.Path, tstr)
	}

	host, _ := os.Hostname()

	return fmt.Sprintf("REMOTE.%s.%d.%s-%s.%s.%s", repoSyncJobName, job.GetNumber(), host, j.Host, j.Path, tstr)
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
				fmt.Sprintf("cd %s && ", j.Path) + fmt.Sprintf(syncCmdTemplate, j.ID()),
			})
	}

	for _, cmd := range j.PreHook {
		args, err := shlex.Split(cmd)
		if err != nil {
			j.AddError(err)
			continue
		}

		cmds = append(cmds, args)
	}

	cmds = append(cmds,
		[]string{"git", "add", "-A"},
		[]string{"git", "pull", "--keep", "--rebase", "--autostash", "origin", "master"},
		[]string{"bash", "-c", "git ls-files -d | xargs -r git rm --ignore-unmatch --quiet -- "},
		[]string{"git", "add", "-A"},
		[]string{"bash", "-c", fmt.Sprintf("git commit -a -m 'update: (%s)' || true", j.ID())},
		[]string{"git", "push"},
	)

	if !j.isLocal() {
		cmds = append(cmds,
			[]string{"ssh", j.Host, fmt.Sprintf("cd %s && ", j.Path) + fmt.Sprintf(syncCmdTemplate, j.ID())},
			[]string{"git", "pull", "--keep", "--rebase", "--autostash", "origin", "master"},
		)
	}

	for _, cmd := range j.PostHook {
		args, err := shlex.Split(cmd)
		if err != nil {
			j.AddError(err)
			continue
		}

		cmds = append(cmds, args)
	}

	if j.HasErrors() {
		return
	}

	jpm := sardis.GetEnvironment().Jasper()

	for idx, cmd := range cmds {
		jpm.CreateCommand(ctx).ID(j.ID()).Add(cmd).Directory(j.Path).
			SetCombinedSender(level.Debug, grip.GetSender()).
			Prerequisite(func() bool {
				grip.Debug(message.Fields{
					"args":  cmd,
					"path":  j.Path,
					"job":   j.ID(),
					"num":   idx,
					"total": len(cmds),
				})
				return true
			}).Run(ctx)

	}

	grip.Info(message.Fields{
		"op":     "completed repo sync",
		"errors": j.HasErrors(),
		"host":   j.Host,
		"path":   j.Path,
		"id":     j.ID(),
	})
}
