package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/dependency"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
)

type repoStatusJob struct {
	Path     string `bson:"path" json:"path" yaml:"path"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoStatusJobName = "repo-status"

func init() {
	registry.AddJobType(repoStatusJobName, func() amboy.Job { return repoStatusJobFactory() })
}

func repoStatusJobFactory() *repoStatusJob {
	j := &repoStatusJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoStatusJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoStatusJob(path string) amboy.Job {
	j := repoStatusJobFactory()
	j.Path = path
	j.SetID(fmt.Sprintf("%s.%s.%d", repoStatusJobName, path, job.GetNumber()))
	return j
}

func (j *repoStatusJob) Run(ctx context.Context) {
	defer j.MarkComplete()
	if _, err := os.Stat(j.Path); os.IsNotExist(err) {
		j.AddError(fmt.Errorf("cannot check status %s, no repository exists", j.Path))
		return
	}

	cmd := jasper.Context(ctx)

	startAt := time.Now()
	sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		"repo": j.Path,
	})
	sender.SetName(fmt.Sprintf("sardis.%s", repoStatusJobName))
	sender.SetFormatter(send.MakeJSONFormatter())
	logger := grip.NewLogger(sender)

	j.AddError(cmd.CreateCommand(ctx).Priority(level.Debug).
		ID(j.ID()).Directory(j.Path).
		SetOutputSender(level.Debug, sender).
		SetErrorSender(level.Debug, sender).
		Add(getStatusCommandArgs(j.Path)).
		AppendArgs("git", "status", "--short", "--branch").
		Run(ctx))

	j.AddError(j.doOtherStat(logger))

	logger.Debug(message.Fields{
		"op":     "git status",
		"secs":   time.Since(startAt).Seconds(),
		"errors": j.HasErrors(),
	})
}

func (j *repoStatusJob) doOtherStat(logger grip.Logger) error {
	repo, err := git.PlainOpen(j.Path)
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	stat, err := wt.Status()
	if err != nil {
		return err
	}

	for fn, status := range stat {
		logger.Notice(message.Fields{
			"file":     fn,
			"stat":     "golib",
			"staging":  status.Staging,
			"worktree": status.Worktree,
		})
	}
	return nil
}

func getStatusCommandArgs(path string) []string {
	return []string{
		"git", "log", "--date=relative", "--decorate", "-n", "1",
		fmt.Sprintf("--format=%s:", filepath.Base(path)) + `%N (%cr) "%s"`,
	}

}
