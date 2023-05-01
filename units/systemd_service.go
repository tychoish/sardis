package units

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func NewSystemServiceSetupJob(conf sardis.SystemdServiceConf) fun.WorkerFunc {
	return func(ctx context.Context) error {
		ec := &erc.Collector{}

		jasper := jasper.Context(ctx)
		cmd := jasper.CreateCommand(ctx)
		sender := grip.Context(ctx).Sender()

		cmd.ID(conf.Name).SetOutputSender(level.Info, sender).
			SetErrorSender(level.Warning, sender).
			Sudo(conf.System)

		switch {
		case conf.User && conf.Enabled:
			cmd.AppendArgs("systemctl", "--user", "enable", conf.Unit)
			if conf.Start {
				cmd.AppendArgs("systemctl", "--user", "start", conf.Unit)
			}
		case conf.User && conf.Disabled:
			cmd.AppendArgs("systemctl", "--user", "disable", conf.Unit)
			cmd.AppendArgs("systemctl", "--user", "stop", conf.Unit)
		case conf.System && conf.Enabled:
			cmd.AppendArgs("systemctl", "enable", conf.Unit)
			if conf.Start {
				cmd.AppendArgs("systemctl", "start", conf.Unit)
			}
		case conf.System && conf.Disabled:
			cmd.AppendArgs("systemctl", "disable", conf.Unit)
			cmd.AppendArgs("systemctl", "stop", conf.Unit)
		default:
			if err := conf.Validate(); err != nil {
				return err
			}
		}

		ec.Add(cmd.Run(ctx))

		err := ec.Resolve()

		grip.Notice(message.Fields{
			"name":   conf.Name,
			"unit":   conf.Unit,
			"system": conf.System,
			"err":    err,
		})

		return err
	}
}
