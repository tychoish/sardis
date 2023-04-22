package units

import (
	"context"
	"fmt"

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

type setupServiceJob struct {
	Conf     sardis.SystemdServiceConf `bson:"conf" json:"conf" yaml:"conf"`
	job.Base `bson:"metadata" json:"metadata" yaml:"metadata"`
}

const setupServiceJobName = "systemd-service-setup"

func init() {
	registry.AddJobType(setupServiceJobName, func() amboy.Job { return setupServiceJobFactory() })
}

func setupServiceJobFactory() *setupServiceJob {
	j := &setupServiceJob{
		Base: job.Base{
			JobType: amboy.JobType{
				Name:    setupServiceJobName,
				Version: 0,
			},
		},
	}
	j.SetDependency(dependency.NewAlways())
	return j
}

func NewSystemServiceSetupJob(conf sardis.SystemdServiceConf) amboy.Job {
	j := setupServiceJobFactory()
	j.Conf = conf
	j.SetID(fmt.Sprintf("%s.%s.%d", j.JobType.Name, conf.Name, job.GetNumber()))
	return j
}

func (j *setupServiceJob) Run(ctx context.Context) {
	defer j.MarkComplete()

	env := sardis.GetEnvironment(ctx)
	jasper := env.Jasper()
	cmd := jasper.CreateCommand(ctx)

	sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		"job": j.ID(),
	})

	cmd.ID(fmt.Sprint(j.ID(), ".", j.Conf.Name)).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Warning, sender).
		Sudo(j.Conf.System)

	switch {
	case j.Conf.User && j.Conf.Enabled:
		cmd.AppendArgs("systemctl", "--user", "enable", j.Conf.Unit)
		if j.Conf.Start {
			cmd.AppendArgs("systemctl", "--user", "start", j.Conf.Unit)
		}
	case j.Conf.User && j.Conf.Disabled:
		cmd.AppendArgs("systemctl", "--user", "disable", j.Conf.Unit)
		cmd.AppendArgs("systemctl", "--user", "stop", j.Conf.Unit)
	case j.Conf.System && j.Conf.Enabled:
		cmd.AppendArgs("systemctl", "enable", j.Conf.Unit)
		if j.Conf.Start {
			cmd.AppendArgs("systemctl", "start", j.Conf.Unit)
		}
	case j.Conf.System && j.Conf.Disabled:
		cmd.AppendArgs("systemctl", "disable", j.Conf.Unit)
		cmd.AppendArgs("systemctl", "stop", j.Conf.Unit)
	default:
		j.AddError(j.Conf.Validate())
		return
	}

	j.AddError(cmd.Run(ctx))

	grip.Notice(message.Fields{
		"job":    j.ID(),
		"name":   j.Conf.Name,
		"unit":   j.Conf.Unit,
		"system": j.Conf.System,
		"err":    j.Error(),
	})
}
