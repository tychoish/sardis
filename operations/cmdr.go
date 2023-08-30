package operations

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func Commander() *cmdr.Commander {
	return cmdr.MakeRootCommander().
		Flags(cmdr.FlagBuilder(false).SetName("jsonLog").SetUsage("format logs as json").Flag(),
			cmdr.FlagBuilder(false).SetName("colorJsonLog").SetUsage("colorized json logs").Flag(),
			cmdr.FlagBuilder(false).SetName("quietStdOut").SetUsage("don't log to standard out").Flag(),
			cmdr.FlagBuilder(false).SetName("quietSyslog", "qs").SetUsage("don't log to syslog").Flag(),
			cmdr.FlagBuilder(filepath.Join(util.GetHomedir(), ".sardis.yaml")).
				SetName("conf", "c").
				SetUsage("configuration file path").
				SetValidate(func(in string) error {
					if in == "" || util.FileExists(in) {
						return nil
					}
					return fmt.Errorf("config file %q does not exist", in)

				}).Flag(),
			cmdr.FlagBuilder("info").
				SetName("level").
				SetUsage("specify logging threshold: emergency|alert|critical|error|warning|notice|info|debug").
				SetValidate(func(val string) error {
					priority := level.FromString(val)
					if priority == level.Invalid {
						return fmt.Errorf("%q is not a valid logging level", val)
					}
					grip.Sender().SetPriority(priority)
					return nil
				}).Flag()).
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetMiddleware(sardis.ContextSetup(
				sardis.WithConfiguration,
				sardis.WithAppLogger,
			)).Add).
		Middleware(sardis.WithDesktopNotify).
		Middleware(func(ctx context.Context) context.Context {
			jpm := jasper.NewManager(jasper.ManagerOptionSetSynchronized())
			srv.AddCleanup(ctx, jpm.Close)
			return jasper.WithManager(ctx, jpm)
		}).
		SetAction(func(ctx context.Context, cc *cli.Context) error {
			return cli.ShowAppHelp(cc)
		}).
		Subcommanders(
			Admin(),
			ArchLinux(),
			Blog(),
			DMenu(),
			Jira(),
			Notify(),
			Repo(),
			RunCommand(),
			Tweet(),
			Utilities(),
			Version(),
		)
}
