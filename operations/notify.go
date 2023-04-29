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

func Notify(ctx context.Context) cli.Command {
	ctx = sardis.WithDesktopNotify(ctx)

	return cli.Command{
		Name:    "notify",
		Aliases: []string{"xmpp"},
		Usage:   "send an xmpp message",
		Subcommands: []cli.Command{
			notifyPipe(ctx),
			notifySend(ctx),
			notifyDesktop(ctx),
		},
	}
}

func notifyPipe(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "pipe",
		Usage:  "send the contents of standard input over xmpp",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)
			ctx = sardis.WithRemoteNotify(ctx, env.Configuration())
			logger := sardis.RemoteNotify(ctx)

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				logger.Notice(message.MakeString(scanner.Text()))
			}
			return nil
		},
	}
}

func notifySend(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "send",
		Usage:  "send the remaining arguments over xmpp",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)
			ctx = sardis.WithRemoteNotify(ctx, env.Configuration())
			sardis.RemoteNotify(ctx).Notice(strings.Join(c.Args(), " "))
			return nil
		},
	}
}
func notifyDesktop(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "desktop",
		Usage:  "send the remaining arguments over xmpp",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			sardis.DesktopNotify(ctx).Notice(strings.Join(c.Args(), " "))
			return nil
		},
	}
}
