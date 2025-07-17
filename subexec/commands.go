package subexec

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/global"
)

type Command struct {
	Name            string                 `bson:"name" json:"name" yaml:"name"`
	GroupName       string                 `bson:"-" json:"-" yaml:"-"`
	Aliases         []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory       string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment     dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	Command         string                 `bson:"command" json:"command" yaml:"command"`
	Commands        []string               `bson:"commands" json:"commands" yaml:"commands"`
	OverrideDefault bool                   `bson:"override_default" json:"override_default" yaml:"override_default"`
	Notify          *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background      *bool                  `bson:"bson" json:"bson" yaml:"bson"`
	Host            *string                `bson:"host" json:"host" yaml:"host"`

	// if possible call the operation rather
	// than execing the commands
	WorkerDefinition fun.Worker
	unaliasedName    string
}

func (conf *Command) NamePrime() string { return ft.Default(conf.unaliasedName, conf.Name) }

func (conf *Command) Worker() fun.Worker {
	if conf.WorkerDefinition != nil {
		return conf.WorkerDefinition
	}

	sender := grip.Sender()
	hn := util.GetHostname()

	return func(ctx context.Context) error {
		return jasper.Context(ctx).CreateCommand(ctx).
			ID(fmt.Sprintf("CMD(%s).HOST(%s).NUM(%d)", conf.Name, util.GetHostname(), 1+len(conf.Commands))).
			Directory(conf.Directory).
			Environment(conf.Environment).
			AddEnv(global.EnvVarSardisLogQuietStdOut, "true").
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Error, sender).
			Background(ft.Ref(conf.Background)).
			Append(conf.Command).
			Append(conf.Commands...).
			Prerequisite(func() bool {
				grip.Info(message.BuildPair().
					Pair("op", conf.Name).
					Pair("host", hn).
					Pair("dir", conf.Directory).
					Pair("cmd", conf.Command).
					Pair("cmds", conf.Commands))
				return true
			}).
			PostHook(func(err error) error {
				desktop := grip.ContextLogger(ctx, global.ContextDesktopLogger)
				if err != nil {
					m := message.WrapError(err, conf.Name)
					desktop.Error(m)
					grip.Critical(err)
					return err
				}
				desktop.Notice(message.Whenln(ft.Ref(conf.Notify), conf.Name, "completed"))
				return nil
			}).Run(ctx)
	}

}
