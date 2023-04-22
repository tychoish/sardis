package operations

import (
	"bufio"
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
		Name:   "pipe",
		Usage:  "send the contents of standard input over xmpp",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			logger := env.Logger()

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
		Name:   "send",
		Usage:  "send the remaining arguments over xmpp",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			sardis.GetEnvironment().Logger().Notice(strings.Join(c.Args(), " "))
			return nil
		},
	}
}
func notifyDesktop() cli.Command {
	return cli.Command{
		Name:   "desktop",
		Usage:  "send the remaining arguments over xmpp",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			sardis.GetEnvironment().Desktop().Notice(strings.Join(c.Args(), " "))
			return nil
		},
	}
}
