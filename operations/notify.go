package operations

import (
	"bufio"
	"os"
	"strings"

	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Notify() cli.Command {
	return cli.Command{
		Name:  "notify, xmpp",
		Usage: "send an xmpp message",
		Subcommands: []cli.Command{
			notifyPipe(),
			notifySend(),
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
			level := grip.GetSender().Level().Default
			logger := env.Logger()

			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				logger.Log(level, message.NewString(scanner.Text()))
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
			env := sardis.GetEnvironment()

			level := grip.GetSender().Level().Threshold
			logger := env.Logger()

			logger.Log(level, strings.Join(c.Args(), " "))

			return nil
		},
	}

}
