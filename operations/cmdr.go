package operations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/global"
	srsrv "github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

/* Project Planning and TODO
- [ ] TODO <cmdr> cut releases of commander
- [ ] TODO <libfun> a log-based Map persistence (sets only, BSON encoding wrapping GOB)
- [X] TODO <sardis> move more (all?) of the operations logic into units, and have a generators produce workers scheme.
- [ ] TODO <fun/libfun> worker pool but be able to pause to let things coalese
- [ ] TODO <cmdr> move to v3 of the cli lib
- [ ] TODO <cmdr> do something with argflags.
- [ ] TODO <cmdr> cmdr.Action adapter for fun.Worker/fnx.Operation
- [x] TODO <fun> finish fn.Converter[T, O] and fn.Filter[T]
- [X] TOOD [fun] WaitGroup should have and Add method that is an fn.Handler for workers/ops
*/

func StringSpecBuilder(flagName string, defaultValue *string) *cmdr.OperationSpec[string] {
	return cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Command) (string, error) {
		if out := cc.String(flagName); out != "" {
			return out, nil
		}

		if out := cc.Args().First(); out != "" {
			return out, nil
		}

		if defaultValue == nil {
			return "", fmt.Errorf("%q is a required flag, and was missing", flagName)
		}

		return stw.DerefZ(defaultValue), nil
	})
}

func ResolveConfiguration(ctx context.Context, cc *cli.Command) (*sardis.Configuration, error) {
	if sardis.HasAppConfiguration(ctx) {
		return sardis.AppConfiguration(ctx), nil
	}
	return LoadConfiguration(cc)
}

func LoadConfiguration(cc *cli.Command) (*sardis.Configuration, error) {
	conf, err := sardis.LoadConfiguration(cc.String("conf"))
	if err != nil {
		return nil, err
	}

	conf.Settings.Logging.Priority = level.FromString(cc.String("level"))
	conf.Settings.Logging.DisableSyslog = stw.Ptr(cc.Bool("quietSyslog") || os.Getenv(global.EnvVarSardisLogQuietSyslog) != "")
	conf.Settings.Logging.DisableStandardOutput = stw.Ptr(cc.Bool("quietStdOut") || os.Getenv(global.EnvVarSardisLogQuietStdOut) != "")
	conf.Settings.Logging.EnableJSONFormating = stw.Ptr(cc.Bool("jsonLog") || os.Getenv(global.EnvVarSardisLogFormatJSON) != "")
	conf.Settings.Logging.EnableJSONColorFormatting = stw.Ptr(cc.Bool("colorJsonLog") || os.Getenv(global.EnvVarSardisLogJSONColor) != "")
	conf.Settings.Runtime.WithAnnotations = cc.Bool("annotate") || os.Getenv(global.EnvVarSardisAnnotate) != "" || conf.Settings.Runtime.AnnotationSeparator != ""
	conf.Settings.Runtime.AnnotationSeparator = util.Default(conf.Settings.Runtime.AnnotationSeparator, global.MenuCommanderDefaultAnnotationSeparator)

	return conf, nil
}

func StandardSardisOperationSpec() *cmdr.OperationSpec[*sardis.Configuration] {
	return cmdr.SpecBuilder(ResolveConfiguration).
		SetMiddleware(
			func(ctx context.Context, conf *sardis.Configuration) context.Context {
				ctx = sardis.WithConfiguration(ctx, conf)
				ctx = subexec.WithJasper(ctx, &conf.Operations)
				ctx = srsrv.WithAppLogger(ctx, conf.Settings.Logging)
				ctx = srsrv.WithRemoteNotify(ctx, conf.Settings)
				return ctx
			})
}

func Commander() *cmdr.Commander {
	return cmdr.MakeRootCommander().
		SetName(global.ApplicationName).
		EnableCompletionCmd().
		Flags(cmdr.FlagBuilder(false).SetName("jsonLog").SetUsage("format logs as json").Flag(),
			cmdr.FlagBuilder(false).SetName("colorJsonLog").SetUsage("colorized json logs").Flag(),
			cmdr.FlagBuilder(false).SetName("quietStdOut").SetUsage("don't log to standard out").Flag(),
			cmdr.FlagBuilder(false).SetName("quietSyslog", "qs").SetUsage("don't log to syslog").Flag(),
			cmdr.FlagBuilder(filepath.Join(util.GetHomeDir(), ".sardis.yaml")).
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
		Middleware(srsrv.WithDesktopNotify).
		Middleware(func(ctx context.Context) context.Context {
			return srv.SetWorkerPool(ctx,
				global.ApplicationName,
				pubsub.NewUnlimitedQueue[fnx.Worker](),
				wpa.WorkerGroupConfWorkerPerCPU(),
				wpa.WorkerGroupConfContinueOnError(),
				wpa.WorkerGroupConfContinueOnPanic(),
			)
		}).
		Middleware(srv.WithCleanup).
		SetAction(func(ctx context.Context, cc *cli.Command) error {
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
			ExecCommand(),
			RunCommand(),
			Tweet(),
			Utilities(),
			Version(),
			SearchMenu(),
		)
}
