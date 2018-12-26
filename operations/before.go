package operations

import (
	"errors"

	"github.com/mongodb/grip"
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
