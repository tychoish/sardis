package subexec

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/global"
	"github.com/tychoish/sardis/util"
)

type Command struct {
	Name            string                  `bson:"name" json:"name" yaml:"name"`
	GroupName       string                  `bson:"-" json:"-" yaml:"-"`
	GroupCategory   string                  `bson:"-" json:"-" yaml:"-"`
	Directory       string                  `bson:"directory" json:"directory" yaml:"directory"`
	Environment     stw.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	Command         string                  `bson:"command" json:"command" yaml:"command"`
	Commands        []string                `bson:"commands" json:"commands" yaml:"commands"`
	OverrideDefault bool                    `bson:"override_default" json:"override_default" yaml:"override_default"`
	Notify          *bool                   `bson:"notify,omitempty" json:"notify,omitempty" yaml:"notify,omitempty"`
	Background      *bool                   `bson:"background,omitempty" json:"background,omitempty" yaml:"background,omitempty"`
	SortHint        int                     `bson:"sort_hint,omitempty" json:"sort_hint,omitempty" yaml:"sort_hint,omitempty"`
	Logs            Logging                 `bson:"logs" json:"logs" yaml:"logs"`

	// if possible call the operation rather
	// than execing the commands
	WorkerDefinition fnx.Worker `bson:"-" json:"-" yaml:"-"`
	unaliasedName    string
}

func (conf *Command) NamePrime() string { return util.Default(conf.unaliasedName, conf.Name) }
func (conf *Command) FQN() string {
	return util.DotJoin(conf.GroupCategory, conf.GroupName, conf.NamePrime())
}

func (conf *Command) Worker() fnx.Worker {
	if conf.WorkerDefinition != nil {
		return conf.WorkerDefinition
	}

	hn := util.GetHostname()
	nonce := strings.ToLower(rand.Text())[:7]
	jobID := fmt.Sprintf("CMD(%s).HOST(%s).NUM(%d)", conf.Name, hn, 1+len(conf.Commands))
	ec := &erc.Collector{}

	return func(ctx context.Context) error {
		proclog, buf := NewOutputBuf(fmt.Sprint(jobID, ".", nonce))
		startAt := time.Now()
		return jasper.Context(ctx).CreateCommand(ctx).
			ID(jobID).
			Directory(conf.Directory).
			Environment(conf.Environment).
			AddEnv(global.EnvVarSardisLogQuietStdOut, "true").
			SetOutputSender(level.Info, buf).
			SetErrorSender(level.Error, buf).
			Background(stw.DerefZ(conf.Background)).
			Append(conf.Command).
			Append(conf.Commands...).
			Prerequisite(func() bool {
				msg := message.BuildKV().
					KV("op", conf.Name).
					KV("state", "STARTED").
					KV("host", hn).
					KV("dir", conf.Directory).
					KV("cmd", conf.Command)

				if len(conf.Commands) > 0 {
					msg.KV("cmds", conf.Commands)
				}

				grip.Info(msg)

				return true
			}).
			// END jasper command definition
			PostHook(func(err error) error {
				ec.Push(err)
				err = ec.Resolve()

				defer util.DropErrorOnDefer(buf.Close)
				msg := message.BuildKV().
					KV("op", conf.Name).
					KV("state", "COMPLETED").
					KV("dur", time.Since(startAt)).
					KV("err", err != nil).
					KV("host", hn).
					KV("dir", conf.Directory).
					KV("cmd", conf.Command)

				if len(conf.Commands) > 0 {
					msg.KV("cmds", conf.Commands)
				}

				defer grip.Notice(msg)

				desktop := grip.ContextLogger(ctx, global.ContextDesktopLogger)
				proclog.Infoln("<---------------", nonce, "---", jobID, "----")
				if err != nil {
					m := message.WrapError(err, conf.Name)
					desktop.Error(m)
					grip.Critical(err)

					grip.Error(buf.String())
					return err
				} else if conf.Logs.Full() {
					grip.Info(buf.String())
				}
				desktop.Notice(message.Whenln(stw.DerefZ(conf.Notify), conf.Name, "completed"))
				return nil
			}).
			Worker().
			PreHook(func(context.Context) {
				proclog.Infoln("----------------", nonce, "---", jobID, "--->")
			}).Run(ctx)
	}
}
