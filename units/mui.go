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
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
)

type mailUpdatederUnit struct {
	MailDirPath     string `bson:"maildir_path" json:"maildir_path" yaml:"maildir_path"`
	MuHome          string `bson:"mu_home_path" json:"mu_home_path" yaml:"mu_home_path"`
	EmacsDaemonName string `bson:"emacs_daemon" json:"emacs_daemon" yaml:"emacs_daemon"`
	Rebuild         bool   `bson:"rebuild" json:"rebuild" yaml:"rebuild"`
	job.Base        `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const muUpdaterJobTypeName = "mu-updater"

func init() {
	registry.AddJobType(muUpdaterJobTypeName, muUpdaterFactory)
}

func muUpdaterFactory() amboy.Job {
	j := &mailUpdatederUnit{
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
	j := muUpdaterFactory().(*mailUpdatederUnit)

	// TODO add verification of these values

	j.MailDirPath = mailDir
	j.MuHome = muHome
	j.EmacsDaemonName = daemonName
	j.Rebuild = rebuild

	j.SetID(fmt.Sprintf("%s-%d-%s-%s", j.Type().Name, job.GetNumber(),
		time.Now().Format("2006-01-02::15.04.05"), daemonName))

	return j
}

func (j *mailUpdatederUnit) Run(ctx context.Context) {
	defer j.MarkComplete()

	cmds := [][]string{
		[]string{"mu", "index", "--quiet", "--maildir=" + j.MailDirPath, "--muhome=" + j.MuHome},
		[]string{"emacsclient", "--server-file=" + j.EmacsDaemonName, "-e", "(mu4e-update-index)"},
	}

	if j.Rebuild {
		cmds[0] = append(cmds[0], "--rebuild")
	}

	for idx, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err == nil {
			grip.Info(strings.Join(append(cmd, "->", string(out)), " "))
			break
		}

		if idx == 0 {
			continue
		}
		grip.Info(strings.Join(append(cmd, "->", string(out)), " "))
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
