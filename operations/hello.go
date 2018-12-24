package operations

import (
	"github.com/mongodb/grip"
	"github.com/urfave/cli"
)

// HelloWorld is a commandline entry point for the hello world enry
// point, and is intended as a small example as a starting point and
// to test basic project organization and cli building.
func HelloWorld() cli.Command {
	return cli.Command{
		Name:    "hello",
		Aliases: []string{"hello-world", "hi"},
		Usage:   "A simple hello world example.",
		Flags:   []cli.Flag{},
		Action: func(c *cli.Context) error {
			grip.Notice("hello world!")
			return nil
		},
	}
}
