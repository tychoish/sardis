package operations

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/dupe"
	"github.com/tychoish/sardis/units"
)

func Utilities() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("utility").
		SetUsage("short utility commands").
		Subcommanders(
			setupLinks(),
			diffTrees(),
		)
}

func diffTrees() *cmdr.Commander {
	return cmdr.AddOperation(cmdr.MakeCommander().
		SetName("tree-diff").
		SetUsage("Compare two trees of files"),
		// parse args
		func(ctx context.Context, cc *cli.Context) (dupe.Options, error) {
			op := dupe.OperationDisplay
			if cc.Bool("deleteMatching") {
				op = dupe.OperationDelete
			}

			opts := dupe.Options{
				Target:    cc.String("target"),
				Mirror:    cc.String("mirror"),
				Operation: op,
			}
			args := cc.Args().Slice()
			switch {
			case opts.Target != "" && opts.Mirror != "":
				return opts, nil
			case opts.Target == "" && opts.Mirror == "" && len(args) >= 2:
				opts.Target = args[0]
				opts.Mirror = args[1]
				return opts, nil
			case opts.Target == "" && opts.Mirror != "" && len(args) >= 1:
				opts.Target = args[0]
				return opts, nil
			case opts.Target != "" && opts.Mirror == "" && len(args) >= 1:
				opts.Mirror = args[0]
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

func setupLinks() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("setup-links").
		SetUsage("setup all configured links").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			ec := &erc.Collector{}
			jobs, run := units.SetupWorkers(ec)

			for _, link := range conf.Links {
				jobs.PushBack(units.NewSymlinkCreateJob(link))
			}

			ec.Add(run(ctx))
			return ec.Resolve()
		}).Add)
}
