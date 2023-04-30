package operations

import (
	"context"
	"fmt"

	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Tweet() cli.Command {
	return cli.Command{
		Name:  "tweet",
		Usage: "send a tweet",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "message",
				Usage: "message to tweet",
			},
		},
		Action: func(ctx context.Context, c *cli.Context) error {
			msg := c.String("message")

			if len(msg) > 280 {
				return fmt.Errorf("message is too long [%d]", len(msg))
			}

			ctx = sardis.WithTwitterLogger(ctx, sardis.AppConfiguration(ctx))
			sardis.Twitter(ctx).Info(msg)

			return nil
		},
	}
}
