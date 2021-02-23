package operations

import (
	"github.com/tychoish/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Admin() cli.Command {
	return cli.Command{
		Name: "admin",
		Subcommands: []cli.Command{
			configCheck(),
		},
	}
}

func configCheck() cli.Command {
	return cli.Command{
		Name:   "config",
		Usage:  "validated configuration",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			conf := sardis.GetEnvironment().Configuration()
			err := conf.Validate()
			if err == nil {
				grip.Info("configuration is valid")
			}
			return errors.Wrap(err, "configuration validation error")
		},
	}
}
