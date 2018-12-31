package operations

import (
	"os"

	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func mergeBeforeFuncs(ops ...func(c *cli.Context) error) cli.BeforeFunc {
	return func(c *cli.Context) error {
		catcher := grip.NewBasicCatcher()

		for _, op := range ops {
			catcher.Add(op(c))
		}

		return catcher.Resolve()
	}
}

func addRemanderToStringSliceFlag(name string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		catcher := grip.NewBasicCatcher()
		for _, v := range c.Args() {
			catcher.Add(c.Set(name, v))
		}
		return catcher.Resolve()
	}
}

func requireConfig() cli.BeforeFunc {
	return func(c *cli.Context) error {
		env := sardis.GetEnvironment()
		if env == nil {
			return errors.New("nil environment")
		}
		conf := env.Configuration()
		if conf == nil {
			return errors.New("conf is nil")
		}
		return nil
	}
}

func requirePathExists(flagName string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		path := c.String(flagName)
		if path == "" {
			if c.NArg() != 1 {
				return errors.New("must specify the path to an evergreen configuration")
			}
			path = c.Args().Get(0)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Errorf("configuration file %s does not exist", path)
		}

		return c.Set(flagName, path)
	}
}
