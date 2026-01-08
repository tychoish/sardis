package operations

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/tools/gadget"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli/v2"
)

func Gadget() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("gadget").
		SetUsage(fmt.Sprint(
			"runs go test (+lint +coverage) on a workspace ",
			"all non-flag arguments are passed directly to go test",
		)).
		Flags(cmdr.FlagBuilder("./").
			SetName("module-path", "m").
			SetUsage("path of top-level workpace for gadget to look for go.mod").
			SetValidate(func(path string) error {
				if path == "./" {
					path = ft.Must(os.Getwd())
				}

				if strings.HasSuffix(path, "...") {
					grip.Warningln("module-path (working directory) should not use '...';",
						"set go test path with '--path'")
					return fmt.Errorf("module path %q should not have ... suffix", path)
				}
				if util.FileExists(util.TryExpandHomeDir(path)) {
					return nil
				}
				grip.Warning(fmt.Errorf("%q does not exist", path))
				return nil
			}).Flag(),
			cmdr.FlagBuilder("...").
				SetName("path", "p").
				SetUsage("path to pass to go test without leading slashes.").
				Flag(),
			cmdr.FlagBuilder(false).
				SetName("recursive", "r").
				SetUsage("run recursively in all nested modules. Also ensures the --path ends with '...'").
				Flag(),
			cmdr.FlagBuilder(10*time.Second).
				SetName("timeout", "t").
				SetUsage("timeout to set to each individual go test invocation").
				Flag(),
			cmdr.FlagBuilder(false).
				SetName("build", "compile", "b").
				SetUsage("runs no-op test build for all packages").
				Flag(),
			cmdr.FlagBuilder(false).
				SetName("skip-lint").
				SetUsage("skip golangci-lint").
				Flag(),
			cmdr.FlagBuilder(false).
				SetName("coverage", "cover", "c").
				SetUsage("runs tests with coverage reporting").
				Flag(),
			cmdr.FlagBuilder(runtime.NumCPU()).
				SetName("workers", "jobs", "j").
				SetUsage("number of parallel workers").
				Flag(),
		).
		Middleware(func(ctx context.Context) context.Context {
			jpm := jasper.NewManager(jasper.ManagerOptionSetSynchronized())
			srv.AddCleanup(ctx, jpm.Close)
			return jasper.WithManager(ctx, jpm)
		}).
		With(cmdr.SpecBuilder(
			func(_ context.Context, cc *cli.Context) (gadget.Options, error) {
				opts := gadget.Options{
					Timeout:        cc.Duration("timeout"),
					Recursive:      cc.Bool("recursive"),
					PackagePath:    cc.String("path"),
					RootPath:       cc.String("module-path"),
					CompileOnly:    cc.Bool("build"),
					SkipLint:       cc.Bool("skip-lint"),
					ReportCoverage: cc.Bool("coverage"),
					UseCache:       true,
					GoTestArgs:     cc.Args().Slice(),
					Workers:        cc.Int("workers"),
				}

				if err := opts.Validate(); err != nil {
					return gadget.Options{}, err
				}

				return opts, nil
			},
		).SetAction(gadget.RunTests).Add)
}
