package operations

import (
	"bufio"
	"os"
	"strings"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func Notify() cli.Command {
	return cli.Command{
		Name:  "xmpp",
		Usage: "send an xmpp message",
		Subcommands: []cli.Command{
			notifyPipe(),
			notifySend(),
		},
	}
}

func notifyPipe() cli.Command {
	return cli.Command{
		Name:  "pipe",
		Usage: "send the contents of standard input over xmpp",
		Action: func(c *cli.Context) error {
			if err := configureSender(); err != nil {
				return errors.Wrap(err, "problem configuring sender")
			}

			level := grip.DefaultLevel()
			logger := sardis.GetLogger()
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
		Name:  "send",
		Usage: "send the remaining arguments over xmpp",
		Action: func(c *cli.Context) error {
			if err := configureSender(); err != nil {
				return errors.Wrap(err, "problem configuring sender")
			}

			level := grip.DefaultLevel()
			logger := sardis.GetLogger()

			logger.Log(level, strings.Join(c.Args(), " "))

			return nil
		},
	}

}
