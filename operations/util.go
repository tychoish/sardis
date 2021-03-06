package operations

import (
	"os"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/dupe"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Utilities() cli.Command {
	return cli.Command{
		Name:    "util",
		Aliases: []string{"utility"},
		Usage:   "short utility commands",
		Subcommands: []cli.Command{
			setupLinks(),
			diffTrees(),
		},
	}
}

func diffTrees() cli.Command {
	return cli.Command{
		Name:  "tree-diff",
		Usage: "Compare two trees of files",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "target",
				Usage: "path of (mutable) target directory",
			},
			cli.StringFlag{
				Name:  "mirror",
				Usage: "path of imutable upstream copy",
			},
			cli.BoolFlag{
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

func setupLinks() cli.Command {
	return cli.Command{
		Name:  "setup-links",
		Usage: "setup all configured links",
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			conf := env.Configuration()
			ctx, cancel := env.Context()
			defer cancel()

			queue := env.Queue()
			catcher := grip.NewBasicCatcher()

			for _, link := range conf.Links {
				catcher.Add(queue.Put(ctx, units.NewSymlinkCreateJob(link)))
			}

			amboy.WaitInterval(ctx, queue, 10*time.Millisecond)
			catcher.Add(amboy.ResolveErrors(ctx, queue))

			return catcher.Resolve()
		},
	}

}
