package operations

import (
	"github.com/deciduosity/grip"
	"github.com/tychoish/sardis/dupe"
	"github.com/urfave/cli"
)

func Utilities() cli.Command {
	return cli.Command{
		Name:    "util",
		Aliases: []string{"utility", "utlitities", "utils"},
		Usage:   "short utility commands",
		Subcommands: []cli.Command{
			diffTrees(),
		},
	}
}

func diffTrees() cli.Command {
	return cli.Command{
		Name: "tree-diff",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "target",
				Usage: "path of (mutable) target directory",
			},
			cli.StringFlag{
				Name:  "mirror",
				Usage: "path of imutable upstream copy",
			},
		},
		Before: setMultiPositionalArgs("target", "mirror"),
		Action: func(c *cli.Context) error {
			opts := dupe.Options{
				Target:    c.String("target"),
				Mirror:    c.String("mirror"),
				Semantics: dupe.NameAndContent,
			}

			paths, err := dupe.ListDiffs(opts)
			if err != nil {
				return err
			}

			for _, p := range paths {
				grip.Info(p)
			}
			return nil
		},
	}
}
