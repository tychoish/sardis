package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/system"
	jaspercli "github.com/tychoish/jasper/cli"
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

func buildApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a personal automation tool"
	app.Version = "0.0.1-pre"

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

	const (
		levelFlag         = "level"
		nameFlag          = "name"
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
		cli.StringFlag{
			Name:   nameFlag,
			EnvVar: "SARDIS_LOGGING_NAME",
			Value:  app.Name,
			Usage:  "use to set a different name to the logger",
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

	ctx, cancel := context.WithCancel(context.Background())
	app.Before = func(c *cli.Context) error {
		env := sardis.GetEnvironment()

		path := c.String("conf")
		conf, err := sardis.LoadConfiguration(path)
		if err != nil {
			grip.Warning(fmt.Errorf("problem loading config: %w", err))
			return nil
		}

		conf.Settings.Logging.Name = c.String(nameFlag)
		conf.Settings.Notification.Name = c.String(nameFlag)
		conf.Settings.Logging.DisableStandardOutput = c.Bool(disableFlag)
		conf.Settings.Logging.EnableJSONFormating = c.Bool(jsonFormatingFlag)
		conf.Settings.Logging.Priority = level.FromString(c.String(levelFlag))
		conf.Settings.Notification.Name = conf.Settings.Logging.Name

		loggingSetup(conf.Settings.Logging)

		if err := env.Configure(ctx, conf); err != nil {
			return fmt.Errorf("problem configuring app: %w", err)
		}

		return nil
	}

	app.After = func(c *cli.Context) error {
		err := sardis.GetEnvironment().Close(ctx)
		cancel()
		return err
	}

	return app
}

// logging setup is separate to make it unit testable
func loggingSetup(conf sardis.LoggingConf) {
	grip.Sender().SetName(conf.Name)
	sender := grip.Sender()
	sender.SetName("sardis")
	sender.SetPriority(conf.Priority)
	grip.SetGlobalLogger(grip.NewLogger(sender))

	if conf.EnableJSONFormating {
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	if runtime.GOOS == "linux" {
		sys, err := system.MakeDefault()
		if err != nil {
			return
		}

		if reflect.TypeOf(sys) == reflect.TypeOf(sender) {
			grip.Debug("skipping attempt to mirror logs to systemd/syslog")
			return
		}

		sys.SetName(conf.Name)
		sys.SetPriority(conf.Priority)

		if conf.EnableJSONFormating {
			sys.SetFormatter(send.MakeJSONFormatter())
		}

		if !conf.DisableStandardOutput {
			sender = send.MakeMulti(sys, sender)
		} else {
			sender = sys
		}

		sender.SetName("sardis")
		grip.SetGlobalLogger(grip.NewLogger(sender))
	}
}
