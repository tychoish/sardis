package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/system"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli"
)

func resolveConfiguration(ctx context.Context, cc *cli.Context) (*sardis.Configuration, error) {
	conf, err := sardis.LoadConfiguration(cc.String("conf"))
	if err != nil {
		return nil, err
	}
	conf.Settings.Logging.EnableJSONFormating = cc.Bool("jsonLog")
	conf.Settings.Logging.DisableStandardOutput = cc.Bool("quietStdOut")
	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))

	sender := grip.Sender()
	if runtime.GOOS == "linux" {
		var syslog send.Sender
		syslog, err = system.MakeDefault()
		if err != nil {
			return nil, err
		}

		if conf.Settings.Logging.DisableStandardOutput {
			sender = syslog
		} else {
			sender = send.MakeMulti(syslog, sender)
		}
	}

	if conf.Settings.Logging.EnableJSONFormating {
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	sender.SetName("sardis")
	sender.SetPriority(conf.Settings.Logging.Priority)
	grip.SetGlobalLogger(grip.NewLogger(sender))

	return conf, nil
}

func main() {
	cmd := cmdr.MakeRootCommander().
		SetAppOptions(cmdr.AppOptions{Name: "riker", Usage: "call the opts", Version: "v0.0.1-pre"}).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[string]{
			Name:    "conf, c",
			Usage:   "what to print",
			Default: filepath.Join(util.GetHomeDir(), ".sardis.yaml"),
			Validate: func(in string) error {
				if in == "" || util.FileExists(in) {
					return nil
				}
				return fmt.Errorf("config file %q does not exist", in)
			},
		})).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[string]{
			Name:    "level",
			Default: "info",
			Usage:   "specify logging threshold: emergency|alert|critical|error|warning|notice|info|debug",
		})).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[bool]{Name: "jsonLog", Usage: "format logs as json"})).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[bool]{Name: "quietStdOut", Usage: "don't log to standard out"})).
		AddMiddleware(sardis.WithDesktopNotify).
		AddMiddleware(func(ctx context.Context) context.Context {
			jpm := fun.Must(jasper.NewSynchronizedManager(false))
			srv.AddCleanup(ctx, jpm.Close)
			return jasper.WithManager(ctx, jpm)
		})

	cmdr.AddOperation(cmd, resolveConfiguration,
		func(ctx context.Context, conf *sardis.Configuration) error {
			logger := grip.Context(ctx)
			logger.Notice("hello world")
			logger.Info(jasper.Context(ctx).ID())
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdr.Main(ctx, cmd)
}
