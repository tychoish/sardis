package main

import (
	"context"
	"fmt"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/urfave/cli"
)

func main() {
	cmd := cmdr.MakeRootCommand(context.Background()).
		SetAppOptions(cmdr.AppOptions{Name: "riker", Usage: "call the opts", Version: "v0.0.1-pre"}).
		AddFlag(cmdr.MakeFlag(cmdr.FlagOptions[string]{
			Name:    "print",
			Usage:   "what to print",
			Default: "chair",
			Validate: func(in string) (string, error) {
				return fmt.Sprint(in, in), nil
			},
		})).SetAction(func(ctx context.Context, c *cli.Context) error {
		logger := grip.Context(ctx)
		logger.Notice(c.String("print"))
		return nil
	})

	cmdr.Main(cmd)
}
