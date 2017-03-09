package main

import (
	"fmt"
	"os"

	"github.com/mongodb/grip"
	"github.com/tychoish/sardis/operations"
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
	grip.CatchEmergencyFatal(err)
}

func buildApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "a personal automation tool"
	app.Version = "0.0.1-pre"

	app.Commands = []cli.Command{
		operations.HelloWorld(),
		operations.Notify(),
		operations.Version(),
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
		// TODO log to file/service
	}

	app.Before = func(c *cli.Context) error {
		loggingSetup(app.Name, c.String("level"))
		return nil
	}

	return app
}

// logging setup is separate to make it unit testable
func loggingSetup(name, level string) {
	grip.SetName(name)
	grip.SetThreshold(level)
}
