package units

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/dependency"
	"github.com/deciduosity/amboy/job"
	"github.com/deciduosity/amboy/registry"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
)

type mailUpdatederJob struct {
	MailDirPath     string `bson:"maildir_path" json:"maildir_path" yaml:"maildir_path"`
	MuHome          string `bson:"mu_home_path" json:"mu_home_path" yaml:"mu_home_path"`
	EmacsDaemonName string `bson:"emacs_daemon" json:"emacs_daemon" yaml:"emacs_daemon"`
	Rebuild         bool   `bson:"rebuild" json:"rebuild" yaml:"rebuild"`
	job.Base        `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const muUpdaterJobTypeName = "mu-updater"

func init() { registry.AddJobType(muUpdaterJobTypeName, func() amboy.Job { return muUpdaterFactory() }) }

func muUpdaterFactory() *mailUpdatederJob {
	j := &mailUpdatederJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    muUpdaterJobTypeName,
				Version: 1,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewMailUpdaterJob(mailDir, muHome, daemonName string, rebuild bool) amboy.Job {
	j := muUpdaterFactory()

	j.MailDirPath = mailDir
	j.MuHome = muHome
	j.EmacsDaemonName = daemonName
	j.Rebuild = rebuild

	j.SetID(fmt.Sprintf("%s-%d-%s-%s", j.Type().Name, job.GetNumber(),
		time.Now().Format("2006-01-02::15.04.05"), daemonName))

	return j
}

func (j *mailUpdatederJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	cmds := [][]string{
		[]string{"mu", "index", "--quiet", "--maildir=" + j.MailDirPath, "--muhome=" + j.MuHome},
		[]string{"emacsclient", "--server-file=" + j.EmacsDaemonName, "-e", "(mu4e-update-index)"},
	}

	if j.Rebuild {
		cmds[0] = append(cmds[0], "--rebuild")
	}

	for idx, cmd := range cmds {
		out, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
		grip.Debug(message.Fields{
			"id":   j.ID(),
			"cmd":  strings.Join(cmd, " "),
			"err":  err != nil,
			"path": j.MailDirPath,
			"idx":  idx,
			"num":  len(cmds),
			"out":  strings.Trim(strings.Replace(string(out), "\n", "\n\t out -> ", -1), "\n\t out->"),
		})

		if err == nil {
			break
		}

		if idx == 0 {
			continue
		}

		j.AddError(err)
	}
	grip.Info(message.Fields{
		"op":     "completed updating mail database",
		"id":     j.ID(),
		"path":   j.MailDirPath,
		"emacs":  j.EmacsDaemonName,
		"errors": j.HasErrors(),
	})
}
