package units

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/dependency"
	"github.com/mongodb/amboy/job"
	"github.com/mongodb/amboy/registry"
	"github.com/mongodb/grip"
)

type mailUpdatederUnit struct {
	MailDirPath     string `bson:"maildir_path" json:"maildir_path" yaml:"maildir_path"`
	MuHome          string `bson:"mu_home_path" json:"mu_home_path" yaml:"mu_home_path"`
	EmacsDaemonName string `bson:"emacs_daemon" json:"emacs_daemon" yaml:"emacs_daemon"`
	Rebuild         bool   `bson:"rebuild" json:"rebuild" yaml:"rebuild"`
	*job.Base       `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const muUpdaterJobTypeName = "mu-updater"

func init() {
	registry.AddJobType(muUpdaterJobTypeName, muUpdaterFactory)

}

func muUpdaterFactory() amboy.Job {
	j := &mailUpdatederUnit{
		Base: &job.Base{
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

func (j *mailUpdatederUnit) Run() {
	defer j.MarkComplete()

	cmds := [][]string{
		[]string{"mu", "index", "--maildir=" + j.MailDirPath, "--muhome=" + j.MuHome},
		[]string{"emacsclient", "--server-file=" + j.EmacsDaemonName, "-e", "\"(mu4e-update-index)\""},
	}

	if j.Rebuild {
		cmds[0] = append(cmds[0], "--rebuild")
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			j.AddError(err)
			grip.Error(out)
			return
		}
		grip.Info(strings.Join(cmd, " "))
		grip.Debug(out)
	}
	grip.Infof("completed updating mail database for %s and %s (%s)",
		j.MailDirPath, j.EmacsDaemonName, j.ID())
}
