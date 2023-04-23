package main

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/urfave/cli"
)

func main() {
	cmd := cmdr.MakeCommander().
		SetAppOptions(cmdr.AppOptions{Name: "riker", Usage: "call the opts", Version: "v0.0.1-pre"}).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[string]{
			Name:     "print",
			Usage:    "what to print",
			Default:  "chair",
			Validate: func(in string) error { return nil },
		})).SetAction(
		func(ctx context.Context, c *cli.Context) error {
			logger := grip.Context(ctx)
			logger.Notice(c.String("print"))
			return nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmdr.Main(ctx, cmd)
}
