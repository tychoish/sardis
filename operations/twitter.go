package operations

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/sardis"
)

func Tweet() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("tweet").
		SetUsage("send a tweet").
		Flags(cmdr.FlagBuilder("").
			SetName("message", "m").
			SetUsage("message to tweet").
			Flag()).
		With(cmdr.SpecBuilder(
			func(ctx context.Context, cc *cli.Context) (string, error) {
				return cc.String("message"), nil
			}).
			SetAction(func(ctx context.Context, msg string) error {
				if len(msg) > 280 {
					return fmt.Errorf("message is too long [%d]", len(msg))
				}

				ctx = sardis.WithTwitterLogger(ctx, sardis.AppConfiguration(ctx))
				sardis.Twitter(ctx).Info(msg)

				return nil
			}).Add)
}
