package units

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/dependency"
	"github.com/tychoish/amboy/job"
	"github.com/tychoish/amboy/registry"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/sardis"
)

type repoFetchJob struct {
	Conf     *sardis.RepoConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const repoFetchJobName = "repo-fetch"

func init() { registry.AddJobType(repoFetchJobName, func() amboy.Job { return repoFetchFactory() }) }

func repoFetchFactory() *repoFetchJob {
	j := &repoFetchJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    repoFetchJobName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewRepoFetchJob(conf *sardis.RepoConf) amboy.Job {
	j := repoFetchFactory()

	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%d.%s", repoFetchJobName, job.GetNumber(), j.Conf.Path))
	return j
}

func stringSliceContains(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}

	}
	return false
}

func (j *repoFetchJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	if _, err := os.Stat(j.Conf.Path); os.IsNotExist(err) {
		grip.Info(message.Fields{
			"id":   j.ID(),
			"path": j.Conf.Path,
			"op":   "repo doesn't exist; cloning",
		})

		job := NewRepoCloneJob(j.Conf)
		job.Run(ctx)
		j.AddError(job.Error())
		return
	}

	if j.Conf.RemoteName == "" || j.Conf.Branch == "" {
		j.AddError(errors.New("repo-fetch requires defined remote name and branch for the repo"))
		return
	}
	env := sardis.GetEnvironment(ctx)
	conf := env.Configuration()
	cmd := env.Jasper().CreateCommand(ctx)

	sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		"job":  j.ID(),
		"repo": j.Conf.Name,
	})

	cmd.ID(j.ID()).Directory(j.Conf.Path).
		AddEnv(sardis.SSHAgentSocketEnvVar, conf.Settings.SSHAgentSocketPath).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Warning, sender)

	if j.Conf.LocalSync && stringSliceContains("mail", j.Conf.Tags) {
		cmd.Append(j.Conf.Pre...)
	}

	cmd.AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", j.Conf.RemoteName, j.Conf.Branch)
	cmd.Append(j.Conf.Post...)

	j.AddError(cmd.Run(ctx))

	grip.Notice(message.Fields{
		"path":   j.Conf.Path,
		"repo":   j.Conf.Remote,
		"errors": j.HasErrors(),
		"op":     "repo fetch",
	})
}
