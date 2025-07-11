package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

/* Project Planning and TODO
- [ ] TODO <cmdr> cut releases of commander
- [ ] TODO <libfun> a log-based Map persistence (sets only, BSON encoding wrapping GOB)
- [ ] TODO <sardis> move more (all?) of the operations logic into units, and have a generators produce workers scheme.
- [ ] TODO <fun/libfun> worker pool but be able to pause to let things coalese
- [ ] TODO <cmdr> move to v3 of the cli lib
- [ ] TODO <cmdr> do something with argflags.
- [ ] TODO <cmdr> cmdr.Action adapter for fun.Worker/fun.Operation
*/

func StringSpecBuilder(flagName string, defaultValue *string) *cmdr.OperationSpec[string] {
	return cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (string, error) {
		if out := cc.String(flagName); out != "" {
			return out, nil
		}

		if out := cc.Args().First(); out != "" {
			return out, nil
		}

		if defaultValue == nil {
			return "", fmt.Errorf("%q is a required flag, and was missing", flagName)
		}

		return ft.Ref(defaultValue), nil
	})
}

func ResolveConfiguration(ctx context.Context, cc *cli.Context) (*sardis.Configuration, error) {
	if sardis.HasAppConfiguration(ctx) {
		return sardis.AppConfiguration(ctx), nil
	}

	conf, err := sardis.LoadConfiguration(cc.String("conf"))
	if err != nil {
		return nil, err
	}

	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))

	conf.Settings.Logging.DisableSyslog = cc.Bool("quietSyslog") || os.Getenv(sardis.EnvVarSardisLogQuietSyslog) != ""
	conf.Settings.Logging.DisableStandardOutput = cc.Bool("quietStdOut") || os.Getenv(sardis.EnvVarSardisLogQuietStdOut) != ""
	conf.Settings.Logging.EnableJSONFormating = cc.Bool("jsonLog") || os.Getenv("SARDIS_LOG_FORMAT_JSON") != ""
	conf.Settings.Logging.EnableJSONColorFormatting = cc.Bool("colorJsonLog") || os.Getenv("SARDIS_LOG_COLOR_JSON") != ""

	return conf, nil
}

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
		Middleware(func(ctx context.Context) context.Context {
			return srv.SetWorkerPool(ctx,
				sardis.ApplicationName,
				pubsub.NewUnlimitedQueue[fun.Worker](),
				fun.WorkerGroupConfWorkerPerCPU(),
				fun.WorkerGroupConfContinueOnError(),
				fun.WorkerGroupConfContinueOnPanic(),
			)
		}).
		SetAction(func(ctx context.Context, cc *cli.Context) error {
			return cli.ShowAppHelp(cc)
		}).
		Subcommanders(
			Admin(),
			ArchLinux(),
			Blog(),
			DMenu(),
			Gadget(),
			Jira(),
			Notify(),
			Repo(),
			RunCommand(),
			Tweet(),
			Utilities(),
			Version(),
		)
}
