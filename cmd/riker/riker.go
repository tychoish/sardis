package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/daggen"
	"github.com/tychoish/sardis/gadget"
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
		Subcommanders(
			cmdr.MakeCommander().SetName("daggen").
				Flags(cmdr.FlagBuilder("./").
					SetName("path").
					SetValidate(func(path string) error {
						toCheck := util.TryExpandHomedir(path)
						if strings.HasSuffix(path, "...") {
							toCheck = filepath.Dir(path)
						}
						if util.FileExists(toCheck) {
							return nil
						}
						grip.Warning(fmt.Errorf("%q does not exist", path))
						return nil
					}).Flag()).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					boops, err := daggen.Collect(ctx, cc.String("path"))
					if err != nil {
						return err
					}

					if n, err := boops.WriteTo(os.Stdout); err != nil {
						return fmt.Errorf("writing to stdout %d: %w", n, err)
					}

					return nil
				}),
			cmdr.MakeCommander().SetName("gadget").
				Flags(cmdr.FlagBuilder("./").
					SetName("path").
					SetValidate(func(path string) error {
						toCheck := util.TryExpandHomedir(path)
						if strings.HasSuffix(path, "...") {
							toCheck = filepath.Dir(path)
						}
						if util.FileExists(toCheck) {
							return nil
						}
						grip.Warning(fmt.Errorf("%q does not exist", path))
						return nil
					}).Flag()).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					if err := gadget.RunTests(ctx, gadget.Options{
						RootPath:  fun.Must(filepath.Abs(cc.String("path"))),
						Path:      "...",
						Timeout:   10 * time.Second,
						Recursive: true,
					}); err != nil {
						return erc.Wrap(err, "gadget return")
					}
					return nil
				}),
		)
}
