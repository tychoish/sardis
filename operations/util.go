package operations

import (
	"context"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/dupe"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli/v2"
)

func Utilities() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("utility").
		SetUsage("short utility commands").
		Subcommanders(setupLinks()).
		UrfaveCommands(diffTrees())
}

func diffTrees() *cli.Command {
	return &cli.Command{
		Name:  "tree-diff",
		Usage: "Compare two trees of files",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "target",
				Usage: "path of (mutable) target directory",
			},
			&cli.StringFlag{
				Name:  "mirror",
				Usage: "path of imutable upstream copy",
			},
			&cli.BoolFlag{
				Name:  "deleteMatching",
				Usage: "when specified delete files from the target that are the same in the mirror",
			},
		},
		Before: setMultiPositionalArgs("target", "mirror"),
		Action: func(c *cli.Context) error {
			shouldDelete := c.Bool("deleteMatching")
			opts := dupe.Options{
				Target: c.String("target"),
				Mirror: c.String("mirror"),
			}

			paths, err := dupe.Find(opts)
			if err != nil {
				return err
			}

			for _, p := range paths {
				grip.Info(p)
				if shouldDelete {
					grip.Warning(os.Remove(p))
				}
			}
			return nil
		},
	}
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
