package operations

import (
	"context"
	"fmt"

	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Tweet(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "tweet",
		Usage: "send a tweet",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "message",
				Usage: "message to tweet",
			},
		},
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)
			msg := c.String("message")

			if len(msg) > 280 {
				return fmt.Errorf("message is too long [%d]", len(msg))
			}

			env.Twitter().Info(msg)

			return nil
		},
	}
}
