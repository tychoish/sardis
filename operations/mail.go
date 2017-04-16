package operations

import (
	"os/user"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func Mail() cli.Command {
	return cli.Command{
		Name:  "mail",
		Usage: "a collections of commands to manage the maildir deployment",
		Subcommands: []cli.Command{
			updateDB(),
		},
	}
}

func updateDB() cli.Command {
	user, err := user.Current()
	grip.CatchWarning(err)

	return cli.Command{
		Name:    "mu-update",
		Aliases: []string{"mu", "mui", "muiw"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "mail",
				Usage: "specify the path to the Maildir",
				Value: "~/mail",
			},
			cli.StringFlag{
				Name:  "mu",
				Usage: "specify the path to the muhome",
				Value: "~/.mu",
			},
			cli.StringFlag{
				Name:  "daemon",
				Usage: "name of emacs deamon",
				Value: user.Username,
			},
			cli.BoolFlag{
				Name:  "rebuild",
				Usage: "should perform a full rebuild of the index",
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := configureQueue(); err != nil {
				return errors.Wrap(err, "problem setting up services")
			}

			queue, err := sardis.GetQueue()
			if err != nil {
				return errors.Wrap(err, "problem fetching queue")
			}

			if err := queue.Start(ctx); err != nil {
				return errors.Wrap(err, "problem starting queue")
			}

			j := units.NewMailUpdaterJob(c.String("mail"), c.String("mu"), c.String("daemon"), c.Bool("rebuild"))
			if err := queue.Put(j); err != nil {
				return errors.Wrap(err, "problem registering job")
			}

			amboy.WaitCtxInterval(ctx, queue, 100*time.Millisecond)

			return errors.Wrap(amboy.ResolveErrors(ctx, queue), "job encountered problem")
		},
	}
}
