package operations

import (
	"bytes"
	"fmt"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/cheynewallace/tabby"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli"
)

func RunCommand() cli.Command {
	const commandFlagName = "command"
	return cli.Command{
		Name:  "run",
		Usage: "runs a predefined command",
		Flags: []cli.Flag{
			cli.StringSliceFlag{
				Name:  joinFlagNames(commandFlagName, "c"),
				Usage: "specify a default flag name",
			},
		},
		Subcommands: []cli.Command{
			listCommands(),
			qrCode(),
		},
		Before: mergeBeforeFuncs(
			requireConfig(),
			requireCommandsSet(commandFlagName),
		),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()

			cmds := conf.ExportCommands()
			ops := c.StringSlice(commandFlagName)
			for idx, name := range ops {
				cmd, ok := cmds[name]
				if !ok {
					return fmt.Errorf("command '%s' [%d/%d] does not exist", name, idx+1, len(ops))
				}
				err := env.Jasper().CreateCommand(ctx).Directory(cmd.Directory).ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(ops))).
					Append(cmd.Command).SetCombinedSender(level.Info, grip.Sender()).
					Prerequisite(func() bool {
						grip.Info(message.Fields{
							"cmd":  name,
							"dir":  cmd.Directory,
							"exec": cmd.Command,
							"num":  idx + 1,
							"len":  len(ops),
						})
						return true
					}).Run(ctx)
				if err != nil {
					return err
				}
			}

			return nil
		},
	}
}

func listCommands() cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "return a list of defined commands",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()

			table := tabby.New()
			table.AddHeader("Name", "Command", "Directory")
			for _, cmd := range env.Configuration().Commands {
				table.AddLine(cmd.Name, cmd.Command, cmd.Directory)
			}

			table.Print()

			return nil
		},
	}
}

type bufCloser struct {
	bytes.Buffer
}

func (b bufCloser) Close() error { return nil }

func qrCode() cli.Command {
	return cli.Command{
		Name:   "qr",
		Usage:  "gets qrcode from x11 clipboard and renders it on the terminal",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()
			buf := &bufCloser{}

			err := env.Jasper().CreateCommand(ctx).
				AppendArgs("xsel", "--clipboard", "--output").SetOutputWriter(buf).
				Run(ctx)
			if err != nil {
				return fmt.Errorf("problem getting clipboard: %w", err)
			}

			grip.Info(buf.String())
			qrcodeTerminal.New().Get(buf.String()).Print()

			return nil
		},
	}
}
