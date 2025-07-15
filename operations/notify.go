package operations

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
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
		With(StandardSardisOperationSpec().
			SetMiddleware(sardis.WithRemoteNotify).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				logger := sardis.RemoteNotify(ctx)

				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					logger.Notice(message.MakeString(scanner.Text()))
				}
				return nil
			}).Add)
}

func notifySend() *cmdr.Commander {
	cmd := cmdr.MakeCommander().
		SetName("send").
		SetUsage("send the remaining arguments over xmpp")
	return addOpCommand(cmd, "message", func(ctx context.Context, args *opsCmdArgs[[]string]) error {
		sardis.RemoteNotify(ctx).Notice(strings.Join(args.ops, " "))
		return nil
	})
}

func notifyDesktop() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("desktop").
		SetUsage("send desktop notification").
		SetAction(func(ctx context.Context, c *cli.Context) error {
			sardis.DesktopNotify(ctx).Notice(strings.Join(c.Args().Slice(), " "))
			return nil
		})
}
