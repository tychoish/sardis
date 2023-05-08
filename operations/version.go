package operations

import (
	"context"
	"fmt"
	"strings"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli/v2"
)

func Version() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("version").
		Aliases("v").
		SetUsage("returns the version and build information of the binary").
		SetAction(func(ctx context.Context, cc *cli.Context) error {
			fmt.Println(strings.Join([]string{
				"name: " + cc.App.Name,
				"build: " + sardis.BuildRevision,
				"version: " + cc.App.Version,
			}, "\n"))

			return nil
		})
}
