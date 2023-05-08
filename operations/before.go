package operations

import (
	"errors"

	"github.com/urfave/cli/v2"
)

func setMultiPositionalArgs(flags ...string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		var lastUsed int
		tail := c.Args().Slice()
		for _, f := range flags {
			if c.IsSet(f) {
				continue
			}

			if len(tail) <= lastUsed {
				return errors.New("insufficient number of arguments specified")
			}

			if err := c.Set(f, tail[lastUsed]); err != nil {
				return err
			}

			lastUsed++
		}

		return nil
	}
}
