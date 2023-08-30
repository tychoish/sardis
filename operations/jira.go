package operations

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
)

func Jira() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("jira").
		SetUsage("a collections of commands for jira management").
		Subcommanders(bulkCreateTickets())
}

func bulkCreateTickets() *cmdr.Commander {
	const pathFlagName = "path"

	return cmdr.MakeCommander().SetName("create").
		SetUsage("create tickets").
		With(cmdr.SpecBuilder(ResolveConfiguration).SetMiddleware(sardis.WithConfiguration).Add).
		Flags(cmdr.FlagBuilder(ft.Must(os.Getwd())).
			SetName(pathFlagName).
			SetUsage("specify the name of a file that holds jira tickets").
			SetValidate(func(path string) error {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					return fmt.Errorf("configuration file %s does not exist", path)
				}
				return nil
			}).
			Flag()).
		With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (string, error) {
			return cc.String(pathFlagName), nil
		}).SetAction(func(ctx context.Context, path string) error {
			conf := sardis.AppConfiguration(ctx)

			if conf.Settings.Credentials.Jira.URL == "" {
				return errors.New("cannot create jira tickets with empty config")
			}

			return units.NewBulkCreateJiraTicketJob(path).Run(ctx)
		}).Add)
}
