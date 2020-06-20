package operations

import (
	"github.com/urfave/cli"
)

func Project() cli.Command {
	return cli.Command{
		Name:  "project",
		Usage: "commands for managing groups of repositories",
		Subcommands: []cli.Command{
			projectStatus(),
		},
	}
}

func projectStatus() cli.Command {
	return cli.Command{
		Name:  "status",
		Usage: "get the status of all repositories in a project",
		Action: func(c *cli.Context) error {
			return nil
		},
	}
}
