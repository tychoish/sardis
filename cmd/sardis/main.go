package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/coreos/go-systemd/journal"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/system"
	"github.com/tychoish/jasper"
	jaspercli "github.com/tychoish/jasper/x/cli"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/operations"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli"
)

func main() {
	// this is where the main action of the program starts. The
	// command line interface is managed by the cli package and
	// its objects/structures. This, plus the basic configuration
	// in buildApp(), is all that's necessary for bootstrapping the
	// environment.
	app := buildApp()
	err := app.Run(os.Args)
	grip.Error(err)
	if err != nil {
		os.Exit(1)
	}
}

func reformCommands(ctx context.Context, cmds []cli.Command) {
	for _, cmd := range cmds {
		switch cc := cmd.Action.(type) {
		case func(*cli.Context) error, nil:
			continue
		case func(context.Context, *cli.Context) error:
			cmd.Action = func(clictx *cli.Context) error {
				return cc(ctx, clictx)
			}
		default:
			grip.Warningf("command has malformed action %s [%T]", cmd.Name, cmd)
		}
		reformCommands(ctx, cmd.Subcommands)
	}
}

func buildApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a personal automation tool"
	app.Version = "0.0.1-pre"

	const (
		levelFlag         = "level"
		disableFlag       = "disableStdOutLogging"
		jsonFormatingFlag = "jsonFormatLogging"
	)

	// These are global options. Use this to configure logging or
	// other options independent from specific sub commands.
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  levelFlag,
			Value: "info",
			Usage: fmt.Sprintln("Specify lowest visible loglevel as string: ",
				"'emergency|alert|critical|error|warning|notice|info|debug'"),
		},
		cli.BoolFlag{
			Name:   disableFlag,
			EnvVar: "SARDIS_LOGGING_DISABLE_STD_OUT",
			Usage: fmt.Sprintln("specify to disable output to standard output.",
				"On non-linux systems this does nothing. ",
			),
		},
		cli.BoolFlag{
			Name:   jsonFormatingFlag,
			EnvVar: "SARDIS_LOGGING_ENABLE_JSON_FORMATTING",
			Usage:  "specify to enable json formating for log messages",
		},
		cli.StringFlag{
			Name:  "conf, c",
			Value: filepath.Join(util.GetHomeDir(), ".sardis.yaml"),
		},
	}

	app.Commands = []cli.Command{
		operations.Notify(),
		operations.Tweet(),
		operations.Version(),
		operations.DMenu(),
		operations.Admin(),
		operations.ArchLinux(),
		operations.Repo(),
		operations.Jira(),
		operations.RunCommand(),
		operations.Blog(),
		operations.Utilities(),
		jaspercli.Jasper(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	ctx = srv.WithOrchestrator(ctx)
	ctx = srv.WithCleanup(ctx)

	app.Before = func(c *cli.Context) error {
		path := c.String("conf")
		conf, err := sardis.LoadConfiguration(path)

		conf.Settings.Logging.DisableStandardOutput = c.Bool(disableFlag)
		conf.Settings.Logging.EnableJSONFormating = c.Bool(jsonFormatingFlag)
		conf.Settings.Logging.Priority = level.FromString(c.String(levelFlag))

		loggingSetup(conf.Settings.Logging)

		ctx = sardis.WithConfiguration(ctx, conf)

		jpm, err := jasper.NewSynchronizedManager(false)
		if err != nil {
			srv.AddCleanupError(ctx, err)
		} else {
			ctx = jasper.WithManager(ctx, jpm)
			srv.AddCleanup(ctx, jpm.Close)
		}

		// reset now so we give things the right context
		reformCommands(ctx, app.Commands)

		return nil
	}

	app.After = func(c *cli.Context) error {
		cancel()
		return srv.GetOrchestrator(ctx).Wait()
	}

	return app

}

// logging setup is separate to make it unit testable
func loggingSetup(conf sardis.LoggingConf) {
	sender := grip.Sender()

	if runtime.GOOS == "linux" {
		var syslog send.Sender
		var err error
		if journal.Enabled() {
			syslog, err = system.MakeSystemdSender()
			if err != nil {
				return
			}
			return
		} else {
			syslog = system.MakeLocalSyslog()
		}

		if !conf.DisableStandardOutput {
			sender = send.MakeMulti(syslog, sender)
		} else {
			sender = syslog
		}
	}

	if conf.EnableJSONFormating {
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	sender.SetName(os.Args[0])
	sender.SetPriority(conf.Priority)
	grip.SetGlobalLogger(grip.NewLogger(sender))
}
