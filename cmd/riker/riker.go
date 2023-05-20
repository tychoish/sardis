package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/util"
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
			cmdr.MakeCommander().
				SetName("daggen").
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
					boops, err := gadget.Collect(ctx, cc.String("path"))
					if err != nil {
						return err
					}

					if n, err := boops.WriteTo(os.Stdout); err != nil {
						return fmt.Errorf("writing to stdout %d: %w", n, err)
					}

					return nil
				}),
			cmdr.MakeCommander().
				SetName("gadget").
				SetUsage(fmt.Sprintln("runs go test (+lint +coverage) on a workspace",
					"all non-flag arguments are passed directly to go test")).
				Flags(
					cmdr.FlagBuilder("./").
						SetName("module-path", "m").
						SetUsage("path of top-level workpace for gadget to look for go.mod").
						SetValidate(func(path string) error {
							if path == "./" {
								path = fun.Must(os.Getwd())
							}

							if strings.HasSuffix(path, "...") {
								grip.Warningln("module-path (working directory) should not use '...';",
									"set go test path with '--path'")
								return fmt.Errorf("module path %q should not have ... suffix", path)
							}
							if util.FileExists(util.TryExpandHomedir(path)) {
								return nil
							}
							grip.Warning(fmt.Errorf("%q does not exist", path))
							return nil
						}).Flag(),
					cmdr.FlagBuilder("...").
						SetName("path", "p").
						SetUsage(fmt.Sprintln("path to pass to go test without leading slashes.",
							"(the same path is passed to all invocations, which doesn't always make ",
							"sense when recursive is true)")).
						SetValidate(func(path string) error {
							if strings.HasPrefix(path, "./") {
								return fmt.Errorf("%q should not have a leading './'", path)
							}
							return nil
						}).
						Flag(),
					cmdr.FlagBuilder(false).
						SetName("recursive").
						SetUsage("run recursively in all nested modules. Also ensures the --path ends with '...'").
						Flag(),
					cmdr.FlagBuilder(10*time.Second).
						SetName("timeout").
						SetUsage("timeout to set to each individual go test invocation").
						Flag(),
				).
				With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (*gadget.Options, error) {
					opts := &gadget.Options{
						Timeout:     cc.Duration("timeout"),
						Recursive:   cc.Bool("recursive"),
						PackagePath: cc.String("path"),
						RootPath:    cc.String("module-path"),
						GoTestArgs:  cc.Args().Slice(),
						Workers:     runtime.NumCPU(),
					}

					if err := opts.Validate(); err != nil {
						return nil, err
					}

					return opts, nil
				}).SetAction(func(ctx context.Context, opts *gadget.Options) error {
					return gadget.RunTests(ctx, *opts)
				}).Add),
			cmdr.MakeCommander().
				SetName("gogentree").
				SetUsage("for a go module, resolve the internal dependency graph and run go generate with dependency awareness").
				Flags(cmdr.FlagBuilder(fun.Must(os.Getwd())).
					SetName("path", "p").
					SetUsage("path").
					SetValidate(func(path string) error {
						if strings.HasPrefix(path, "./") {
							return fmt.Errorf("%q should not have a leading './'", path)
						}
						return nil
					}).
					Flag()).
				With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (string, error) {
					return cc.String("path"), nil
				}).SetAction(func(ctx context.Context, path string) error {
					grip.Infoln("going to run go generate here", path)
					bo, err := gadget.GetBuildOrder(ctx, path)
					if err != nil {
						return err
					}
					return gadget.RunCommand(ctx, bo, 4, []string{"go", "generate", "./"})
				}).Add),
		)
}
