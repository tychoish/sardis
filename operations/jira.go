package operations

import (
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Jira() cli.Command {
	return cli.Command{
		Name:  "jira",
		Usage: "a collections of commands for jira management",
		Subcommands: []cli.Command{
			bulkCreateTickets(),
		},
	}
}

func bulkCreateTickets() cli.Command {
	const pathFlagName = "path"

	return cli.Command{
		Name:  "create",
		Usage: "create tickets",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  pathFlagName,
				Usage: "specify the name of a file that holds jira tickets",
			},
		},
		Before: mergeBeforeFuncs(
			requirePathExists(pathFlagName),
			requireConfig(),
		),
		Action: func(c *cli.Context) error {
			path := c.String(pathFlagName)
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			defer env.Close(ctx)

			conf := env.Configuration()
			if conf.Settings.Credentials.Jira.URL == "" {
				return errors.New("cannot create jira tickets with empty config")
			}

			job := units.NewBulkCreateJiraTicketJob(path)
			job.Run(ctx)

			return errors.Wrap(job.Error(), "problem running job")
		},
	}
}
