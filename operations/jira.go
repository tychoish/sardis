package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Jira(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "jira",
		Usage: "a collections of commands for jira management",
		Subcommands: []cli.Command{
			bulkCreateTickets(ctx),
		},
	}
}

func bulkCreateTickets(ctx context.Context) cli.Command {
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
			requireConfig(ctx),
		),
		Action: func(c *cli.Context) error {
			path := c.String(pathFlagName)
			env := sardis.GetEnvironment(ctx)

			conf := env.Configuration()
			if conf.Settings.Credentials.Jira.URL == "" {
				return errors.New("cannot create jira tickets with empty config")
			}

			job := units.NewBulkCreateJiraTicketJob(path)
			job.Run(ctx)

			return fmt.Errorf("problem running job: %w", job.Error())
		},
	}
}
