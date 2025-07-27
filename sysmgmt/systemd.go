package sysmgmt

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

type SystemdConfiguration struct {
	Services []SystemdService `bson:"services" json:"services" yaml:"services"`

	Settings struct{} `bson:"settings" json:"settings" yaml:"settings"`
}

type SystemdService struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Unit     string `bson:"unit" json:"unit" yaml:"unit"`
	User     bool   `bson:"user" json:"user" yaml:"user"`
	System   bool   `bson:"system" json:"system" yaml:"system"`
	Enabled  bool   `bson:"enabled" json:"enabled" yaml:"enabled"`
	Disabled bool   `bson:"disabled" json:"disabled" yaml:"disabled"`
	Start    bool   `bson:"start" json:"start" yaml:"start"`
}

func (conf *SystemdConfiguration) Validate() error {
	ec := &erc.Collector{}

	for idx := range conf.Services {
		ec.Push(conf.Services[idx].Validate())
	}

	return ec.Resolve()
}

func (c *SystemdService) Validate() error {
	catcher := &erc.Collector{}
	catcher.When(c.Name == "", ers.Error("must specify service name"))
	catcher.Whenf(c.Unit == "", "cannot specify empty unit for %q", c.Name)
	catcher.Whenf((c.User && c.System) || (!c.User && !c.System),
		"must specify either user or service for %q", c.Name)
	catcher.Whenf((c.Disabled && c.Enabled) || (!c.Disabled && !c.Enabled),
		"must specify either disabled or enabled for %q", c.Name)
	return catcher.Resolve()
}

func (conf *SystemdService) Worker() fun.Worker {
	const opName = "sytemd-service-setup"

	return func(ctx context.Context) error {
		hn := util.GetHostname()
		startAt := time.Now()

		nonce := strings.ToLower(rand.Text())[:7]
		jobID := fmt.Sprintf("OP(%s).SERVICE(%s).HOST(%s).UNIT(%s)", conf.Name, hn, opName, conf.Unit)

		jasper := jasper.Context(ctx)
		cmd := jasper.CreateCommand(ctx).ID(jobID)

		proclog, buf := subexec.NewOutputBuf(fmt.Sprint(jobID, ".", nonce))

		cmd.SetOutputSender(level.Info, buf).
			SetErrorSender(level.Error, buf).
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

		msg := message.BuildPair().
			Pair("op", opName).
			Pair("state", "COMPLETED").
			Pair("unit", conf.Unit).
			Pair("system", conf.System).
			Pair("dur", time.Since(startAt)).
			Pair("host", hn)

		proclog.Infoln("----------------", nonce, "---", jobID, "--->")
		if err := cmd.Run(ctx); err != nil {
			proclog.Infoln("<---------------", nonce, "---", jobID, "----")
			grip.Critical(err)

			grip.Error(buf.String())
			grip.Error(msg.Pair("err", err != nil))
			return err
		}

		grip.Notice(msg)

		return nil
	}
}

func (conf *SystemdConfiguration) TaskGroups() dt.Slice[subexec.Group] {
	groups := make(dt.Slice[subexec.Group], 0, len(conf.Services))
	for idx, service := range conf.Services {
		var command string
		var opString string
		if service.User {
			opString = "user"
			command = "systemctl --user"
		} else {
			opString = "system"
			command = "sudo systemctl"
		}

		groups.Add(subexec.Group{
			Category:      "systemd",
			Name:          service.Name,
			Notify:        ft.Ptr(true),
			Synthetic:     true,
			CmdNamePrefix: opString,
			Command:       fmt.Sprint(command, " {{name}} ", service.Unit),
			SortHint:      -64,
			Commands: []subexec.Command{
				{Name: "restart", SortHint: 32},
				{Name: "stop", SortHint: 16},
				{Name: "start", SortHint: 8},
				{Name: "enable", SortHint: 4},
				{Name: "disable", SortHint: -2},
				{
					Name:             "setup",
					WorkerDefinition: conf.Services[idx].Worker(),
					SortHint:         -4,
				},
				{
					Name:            "logs",
					Command:         fmt.Sprintf("alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command journalctl --follow --pager-end --unit %s", service.Unit),
					OverrideDefault: true,
					SortHint:        -2,
				},
				{
					Name:            "status",
					Command:         fmt.Sprintf("alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command %s {{name}} %s", command, service.Unit),
					OverrideDefault: true,
					SortHint:        -3,
				},
			},
		})
	}

	return groups
}
