package operations

import (
	"os"

	"github.com/cdr/grip"
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
				return errors.New("must specify the path to a configuration")
			}
			path = c.Args().Get(0)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			return errors.Errorf("configuration file %s does not exist", path)
		}

		return c.Set(flagName, path)
	}
}

func requireCommandsSet(flagName string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		flg := c.StringSlice(flagName)
		if len(flg) == 0 {
			if c.NArg() == 0 {
				return errors.New("must specify a command name")
			}

			catcher := grip.NewBasicCatcher()
			catcher.Add(c.Set(flagName, c.Args().First()))
			for _, it := range c.Args().Tail() {
				catcher.Add(c.Set(flagName, it))
			}
			return catcher.Resolve()
		}
		return nil
	}
}

func requireStringOrFirstArgSet(flagName string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		value := c.String(flagName)
		if value == "" {
			if c.NArg() != 1 {
				return errors.Errorf("must specify a '%s'", flagName)
			}
			value = c.Args().Get(0)
		}
		return c.Set(flagName, value)
	}
}

func setAllTailArguements(flagName string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		for _, a := range c.Args() {
			if err := c.Set(flagName, a); err != nil {
				return err
			}
		}
		return nil
	}
}

func setMultiPositionalArgs(flags ...string) cli.BeforeFunc {
	return func(c *cli.Context) error {
		var lastUsed int
		tail := c.Args()
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
