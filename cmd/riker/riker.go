package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/operations"
	"github.com/tychoish/sardis/util"
)

func TopLevel() *cmdr.Commander {
	return cmdr.MakeRootCommander().
		SetAppOptions(cmdr.AppOptions{
			Name:    "riker",
			Usage:   "call the opts",
			Version: "v0.0.1-pre",
		}).
		Flags(
			cmdr.FlagBuilder(false).SetName("jsonLog").SetUsage("format logs as json").Flag(),
			cmdr.MakeFlag(&cmdr.FlagOptions[string]{
				Name:    "conf, c",
				Usage:   "configuration",
				Default: filepath.Join(util.GetHomeDir(), ".sardis.yaml"),
				Validate: func(in string) error {
					if in == "" || util.FileExists(in) {
						return nil
					}
					return fmt.Errorf("config file %q does not exist", in)
				},
			}),
			cmdr.MakeFlag(&cmdr.FlagOptions[string]{
				Name:    "level",
				Default: "info",
				Usage:   "specify logging threshold: emergency|alert|critical|error|warning|notice|info|debug",
				Validate: func(val string) error {
					if level.FromString(val) == level.Invalid {
						return fmt.Errorf("%q is not a valid logging level", val)
					}
					return nil
				},
			}),
			cmdr.MakeFlag(&cmdr.FlagOptions[bool]{
				Name:  "quietStdOut",
				Usage: "don't log to standard out",
			}),
		).
		Middleware(
			sardis.WithDesktopNotify,
			func(ctx context.Context) context.Context {
				jpm := jasper.NewManager(jasper.ManagerOptions{Synchronized: true})
				srv.AddCleanup(ctx, jpm.Close)
				return jasper.WithManager(ctx, jpm)
			},
		).
		Subcommanders(
			operations.Admin(),
			operations.ArchLinux(),
		)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := TopLevel()

	cmdr.Main(ctx, cmd)
}
