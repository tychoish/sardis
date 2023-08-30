package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/gadget"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := TopLevel()

	cmdr.Main(ctx, cmd)
}

func TopLevel() *cmdr.Commander {
	return cmdr.MakeRootCommander().
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

					if n, err := boops.Packages.WriteTo(os.Stdout); err != nil {
						return fmt.Errorf("writing to stdout %d: %w", n, err)
					}

					return nil
				}),
			cmdr.MakeCommander().
				SetName("gogentree").
				SetUsage("for a go module, resolve the internal dependency graph and run go generate with dependency awareness").
				Flags(
					cmdr.FlagBuilder(ft.Must(os.Getwd())).
						SetName("path", "p").
						SetUsage("path of go module to run go generate in").
						SetValidate(func(path string) error {
							if strings.HasPrefix(path, "./") {
								return fmt.Errorf("%q should not have a leading './'", path)
							}
							return nil
						}).
						Flag(),
					cmdr.FlagBuilder(false).
						SetName("all", "a").
						SetUsage("run go generate in all packages not just ones with 'go:generate' comments").
						Flag(),
					cmdr.FlagBuilder(false).
						SetName("continue-on-error", "continue").
						SetUsage("runs go generate stages in 'continue on error' model").
						Flag(),
				).
				SetAction(func(ctx context.Context, cc *cli.Context) error {
					path := cc.String("path")
					filterInputTree := !cc.Bool("all")
					continueOnError := !cc.Bool("continue-on-error")
					// TODO:
					//   - make search paths configurable
					//   - factor away the ripgrep iter dance
					bo, err := gadget.GetBuildOrder(ctx, cc.String("path"))
					if err != nil {
						return err
					}

					spec := bo
					if filterInputTree {
						iter := gadget.Ripgrep(ctx, jasper.Context(ctx), gadget.RipgrepArgs{
							Types:       []string{"go"},
							Regexp:      "go:generate",
							Path:        path,
							Directories: true,
						})

						limits := &dt.Set[string]{}
						limits.Populate(bo.Packages.ConvertPathsToPackages(iter))
						spec = bo.Narrow(limits)
					}

					grip.Info(strings.Join(os.Environ(), "\n"))
					return gadget.GoGenerate(ctx,
						jasper.Context(ctx),
						gadget.GoGenerateArgs{
							Spec:            spec,
							SearchPath:      []string{filepath.Join(path, "bin")},
							ContinueOnError: continueOnError,
						})
				}),
		)
}
