package operations

import (
	"fmt"
	"strings"

	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Version() cli.Command {
	return cli.Command{
		Name:  "version",
		Usage: "returns the version and build information of the binary",
		Action: func(c *cli.Context) error {
			fmt.Println(strings.Join([]string{
				"name: " + c.App.Name,
				"build: " + sardis.BuildRevision,
				"version: " + c.App.Version,
			}, "\n"))

			return nil
		},
	}

}
