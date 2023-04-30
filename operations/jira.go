package operations

import (
	"context"
	"errors"
	"fmt"

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
		Before: requirePathExists(pathFlagName),
		Action: func(ctx context.Context, c *cli.Context) error {
			path := c.String(pathFlagName)
			conf := sardis.AppConfiguration(ctx)

			if conf.Settings.Credentials.Jira.URL == "" {
				return errors.New("cannot create jira tickets with empty config")
			}

			job := units.NewBulkCreateJiraTicketJob(path)
			job.Run(ctx)

			return fmt.Errorf("problem running job: %w", job.Error())
		},
	}
}
