package main

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/jasper/x/cli"
	"github.com/tychoish/sardis/operations"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := operations.Commander()
	cmd.SetAppOptions(cmdr.AppOptions{
		Name:    "sardis",
		Usage:   "tychoish automation",
		Version: "v0.0.1-pre",
	}).UrfaveCommands(cli.Jasper())

	cmdr.Main(ctx, cmd)
}
