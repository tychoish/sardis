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
	"github.com/tychoish/sardis/util"
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
			dmenuListCmds(),
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
			terms := conf.ExportTerminalCommands()

			ops := c.StringSlice(commandFlagName)
			var fontSize int
			if util.GetHostname() == "derrida" {
				fontSize = 12
			} else {
				fontSize = 7
			}

			for idx, name := range ops {
				cmd, cmdOk := cmds[name]
				if cmdOk {
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
					continue
				}
				cmd, termOk := terms[name]
				if termOk {
					err := env.Jasper().CreateCommand(ctx).Directory(cmd.Directory).ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(ops))).
						SetCombinedSender(level.Info, grip.Sender()).
						Append(fmt.Sprintln(
							"alacritty",
							"-o", fmt.Sprintf("font.size=%d", fontSize),
							"--title", cmd.Name,
							"--command", cmd.Command,
						)).
						Prerequisite(func() bool {
							grip.Info(message.Fields{
								"type": "term",
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

				if !cmdOk && !termOk {
					return fmt.Errorf("command %q not defined", name)
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
				table.AddLine(cmd.Name, cmd.Command, util.CollapseHomeDir(cmd.Directory))
			}
			table.Print()
			fmt.Println()

			table = tabby.New()
			table.AddHeader("Terminal", "Command")
			for _, term := range env.Configuration().TerminalCommands {
				table.AddLine(term.Name, term.Command)
			}
			table.Print()

			return nil
		},
	}
}
func dmenuListCmds() cli.Command {
	return cli.Command{
		Name:   "dmenu",
		Usage:  "return a list of defined commands",
		Before: requireConfig(),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()

			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()
			cmds := append(conf.TerminalCommands, conf.Commands...)

			buf := &bytes.Buffer{}

			for idx := range append(cmds) {
				buf.Write([]byte(cmds[idx].Name))
				if idx+1 == len(cmds) {
					break
				}
				buf.Write([]byte("\n"))
			}

			return env.Jasper().CreateCommand(ctx).Bash(`cmd=$(dmenu "$@" -i -nb '#000000' -nf '#ffffff' -fn 'Source Code Pro-11') && sardis run $cmd`).SetInput(buf).Run(ctx)
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
