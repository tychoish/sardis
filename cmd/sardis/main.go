package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/pkg/errors"
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
	grip.EmergencyFatal(err)
}

func buildApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a personal automation tool"
	app.Version = "0.0.1-pre"

	app.Commands = []cli.Command{
		operations.HelloWorld(),
		operations.Notify(),
		operations.Version(),
		operations.Mail(),
	}

	// These are global options. Use this to configure logging or
	// other options independent from specific sub commands.
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "level",
			Value: "info",
			Usage: fmt.Sprintln("Specify lowest visible loglevel as string: ",
				"'emergency|alert|critical|error|warning|notice|info|debug'"),
		},
		cli.StringFlag{
			Name:  "conf, c",
			Value: filepath.Join(util.GetHomeDir(), ".sardis.yaml"),
		},
		// TODO log to file/service
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.Before = func(c *cli.Context) error {
		env := sardis.GetEnvironment()

		loggingSetup(app.Name, c.String("level"))

		path := c.String("conf")
		conf, err := sardis.LoadConfiguration(path)
		if err != nil {
			grip.Debug(errors.Wrap(err, "problem loading config"))
			return nil
		}

		if err := env.Configure(ctx, conf); err != nil {
			return errors.Wrap(err, "problem configuring app")
		}

		return nil

	}
	app.After = func(c *cli.Context) error {
		cancel()
		return nil
	}

	return app
}

// logging setup is separate to make it unit testable
func loggingSetup(name, priority string) {
	grip.SetName(name)
	sender := grip.GetSender()
	li := sender.Level()
	li.Threshold = level.FromString(priority)
	sender.SetLevel(li)
}
