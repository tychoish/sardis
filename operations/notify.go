package operations

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli/v2"
)

func Notify() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("notify").
		Aliases("xmpp").
		SetUsage("send an xmpp message").
		Subcommanders(
			notifyDesktop(),
			notifyPipe(),
			notifySend(),
		)
}

func notifyPipe() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("pipe").
		Aliases("xmpp").
		SetUsage("send the contents of standard input over xmpp").
		SetAction(func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			ctx = sardis.WithRemoteNotify(ctx, conf)

			logger := sardis.RemoteNotify(ctx)

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				logger.Notice(message.MakeString(scanner.Text()))
			}
			return nil
		})
}

func notifySend() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("send").
		SetUsage("send the remaining arguments over xmpp").
		SetAction(func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
			ctx = sardis.WithRemoteNotify(ctx, conf)
			notify := sardis.RemoteNotify(ctx)
			notify.Notice(strings.Join(c.Args().Slice(), " "))

			return nil
		})
}
func notifyDesktop() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("desktop").
		SetUsage("send desktop notification").
		SetAction(func(ctx context.Context, c *cli.Context) error {
			ctx = sardis.WithDesktopNotify(ctx)
			sardis.DesktopNotify(ctx).Notice(strings.Join(c.Args().Slice(), " "))
			return nil
		})
}
