package operations

import (
	"bufio"
	"context"
	"os"
	"strings"

	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Notify() cli.Command {
	return cli.Command{
		Name:    "notify",
		Aliases: []string{"xmpp"},
		Usage:   "send an xmpp message",
		Subcommands: []cli.Command{
			notifyPipe(),
			notifySend(),
			notifyDesktop(),
		},
	}
}

func notifyPipe() cli.Command {
	return cli.Command{
		Name:  "pipe",
		Usage: "send the contents of standard input over xmpp",
		Action: func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			ctx = sardis.WithRemoteNotify(ctx, conf)

			logger := sardis.RemoteNotify(ctx)

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				logger.Notice(message.MakeString(scanner.Text()))
			}
			return nil
		},
	}
}

func notifySend() cli.Command {
	return cli.Command{
		Name:  "send",
		Usage: "send the remaining arguments over xmpp",
		Action: func(ctx context.Context, c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)

			ctx = sardis.WithRemoteNotify(ctx, conf)

			notify := sardis.RemoteNotify(ctx)
			notify.Notice(strings.Join(c.Args(), " "))

			return nil
		},
	}
}
func notifyDesktop() cli.Command {
	return cli.Command{
		Name:  "desktop",
		Usage: "send the remaining arguments over xmpp",
		Action: func(ctx context.Context, c *cli.Context) error {
			ctx = sardis.WithDesktopNotify(ctx)
			sardis.DesktopNotify(ctx).Notice(strings.Join(c.Args(), " "))
			return nil
		},
	}
}
