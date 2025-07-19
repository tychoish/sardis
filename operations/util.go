package operations

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis/tools/dupe"
	"github.com/tychoish/sardis/tools/munger"
)

func Utilities() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("utility").
		SetUsage("short utility commands").
		Subcommanders(
			diffTrees(),
			blogConvert(),
		)
}

func blogConvert() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("convert").
		SetUsage("convert a hugo site to markdown from restructured text").
		Flags(cmdr.FlagBuilder("~/src/blog").SetName("path").Flag()).
		With(StringSpecBuilder("path", nil).SetAction(munger.ConvertSite).Add)
}

func diffTrees() *cmdr.Commander {
	return cmdr.AddOperation(cmdr.MakeCommander().
		SetName("tree-diff").
		SetUsage("Compare two trees of files, printing duplicates."),
		// parse args
		func(ctx context.Context, cc *cli.Context) (dupe.Options, error) {
			opts := dupe.Options{
				Target: cc.String("target"),
				Mirror: cc.String("mirror"),
			}

			if cc.Bool("deleteMatching") {
				opts.Operation = dupe.OperationDelete
			} else {
				opts.Operation = dupe.OperationDisplay
			}

			args := cc.Args().Slice()
			switch {
			case opts.Target != "" && opts.Mirror != "":
				return opts, nil
			case opts.Target != "" && opts.Mirror == "" && len(args) >= 1:
				opts.Mirror = args[0]
				return opts, nil
			case opts.Target == "" && opts.Mirror == "" && len(args) >= 2:
				opts.Target = args[0]
				opts.Mirror = args[1]
				return opts, nil
			case opts.Target == "" && opts.Mirror != "" && len(args) >= 1:
				opts.Target = args[0]
				return opts, nil
			default:
				return opts, fmt.Errorf("resolving dupe options: [target=%q, mirror=%q, num_args=%d]",
					opts.Target, opts.Mirror, len(args))
			}

		},
		// entry point/action
		func(ctx context.Context, opts dupe.Options) error {
			paths, err := dupe.Find(opts)
			if err != nil {
				return err
			}

			for _, p := range paths {
				grip.Info(p)
				if opts.Operation == dupe.OperationDelete {
					grip.Warning(os.Remove(p))
				}
			}
			return nil
		},
		cmdr.FlagBuilder("").SetName("target").SetUsage("path of (mutable) target directory").SetValidate(func(path string) error { return nil }).Flag(),
		cmdr.FlagBuilder("").SetName("mirror").SetUsage("path of imutable upstream copy").SetValidate(func(path string) error { return nil }).Flag(),
		cmdr.FlagBuilder(false).SetName("deleteMatching").SetUsage("when specified delete files from the target that are the same in the mirror").Flag(),
	)
}
