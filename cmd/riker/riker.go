package main

import (
	"context"
	"fmt"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/daggen"
	"github.com/tychoish/sardis/operations"
	"github.com/urfave/cli/v2"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := TopLevel()

	cmdr.Main(ctx, cmd)
}

func TopLevel() *cmdr.Commander {
	return operations.Commander().
		SetAppOptions(cmdr.AppOptions{
			Name:    "riker",
			Usage:   "call the opts",
			Version: "v0.0.1-pre",
		}).
		Subcommanders(cmdr.MakeCommander().
			SetName("daggen").
			Flags(cmdr.FlagBuilder("./").
				SetName("path").
				SetValidate(func(path string) error {
					if util.FileExists(path) {
						return nil
					}
					return fmt.Errorf("%q does not exist", path)
				}).Flag()).
			SetAction(func(ctx context.Context, cc *cli.Context) error {
				daggen.GetDag(ctx, cc.String("path"))
				return nil
			}),
		)
}
