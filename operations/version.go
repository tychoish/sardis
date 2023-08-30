package operations

import (
	"context"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
)

func Version() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("version").
		Aliases("v").
		SetUsage("returns the version and build information of the binary").
		SetAction(func(ctx context.Context, cc *cli.Context) error {
			grip.Log(grip.Sender().Priority(), message.Fields{
				"name":    cc.App.Name,
				"build":   sardis.BuildRevision,
				"version": cc.App.Version,
			})

			return nil
		})
}
