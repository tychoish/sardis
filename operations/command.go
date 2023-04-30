package operations

import (
	"bytes"
	"context"
	"fmt"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/cheynewallace/tabby"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli"
)

const commandFlagName = "command"

func RunCommand(ctx context.Context) cli.Command {
	ctx = sardis.WithDesktopNotify(ctx)

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
			listCommands(ctx),
			dmenuListCmds(ctx, dmenuListCommandAll),
			qrCode(ctx),
		},
		Before: mergeBeforeFuncs(
			requireConfig(ctx),
			requireCommandsSet(commandFlagName),
		),
		Action: func(c *cli.Context) error {
			ops := c.StringSlice(commandFlagName)
			conf := sardis.AppConfiguration(ctx)

			return runConfiguredCommand(ctx, conf, ops)
		},
	}
}

func runConfiguredCommand(ctx context.Context, conf *sardis.Configuration, ops []string) error {
	cmds := conf.ExportAllCommands()

	notify := sardis.DesktopNotify(ctx)

	for idx, name := range ops {
		cmd, ok := cmds[name]
		if !ok {
			return fmt.Errorf("command name %q is not defined", name)
		}
		err := jasper.Context(ctx).CreateCommand(ctx).Directory(cmd.Directory).ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(ops))).
			SetOutputSender(level.Info, grip.Sender()).
			SetErrorSender(level.Error, grip.Sender()).
			Append(cmd.Command).
			Prerequisite(func() bool {
				grip.Info(message.Fields{
					"cmd":  name,
					"dir":  cmd.Directory,
					"exec": cmd.Command,
					"num":  idx + 1,
					"len":  len(ops),
				})
				return true
			}).
			PostHook(func(err error) error {
				if err != nil {
					notify.Error(message.WrapError(err, name))
					return err
				}

				notify.Noticeln(name, "completed")
				return nil
			}).
			Run(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func listCommands(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "list",
		Usage:  "return a list of defined commands",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
			homedir := util.GetHomeDir()

			table := tabby.New()
			table.AddHeader("Name", "Group", "Command", "Directory")
			for _, group := range conf.Commands {
				for _, cmd := range group.Commands {

					if cmd.Directory == homedir {
						cmd.Directory = ""
					}

					switch {
					case cmd.Alias != "":
						table.AddLine(cmd.Name, group.Name, cmd.Alias, cmd.Directory)
					default:
						table.AddLine(cmd.Name, group.Name, cmd.Command, cmd.Directory)
					}
				}
			}
			table.Print()

			return nil
		},
	}
}

type dmenuListCommandTypes int

const (
	dmenuListCommandAll dmenuListCommandTypes = iota
	dmenuListCommandGroup
	dmenuListCommandRun
)

func dmenuListCmds(ctx context.Context, kind dmenuListCommandTypes) cli.Command {
	return cli.Command{
		Name:   "dmenu",
		Usage:  "return a list of defined commands",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			conf := sardis.AppConfiguration(ctx)
			var cmds []sardis.CommandConf

			switch kind {
			case dmenuListCommandAll:
				allCmd := conf.ExportAllCommands()
				for _, cmd := range allCmd {
					cmds = append(cmds, cmd)
				}
			case dmenuListCommandRun:
				allCmd := conf.ExportAllCommands()
				for _, cmd := range allCmd {
					if cmd.Name != "" {
						cmds = append(cmds, cmd)
					}
				}
			case dmenuListCommandGroup:
				for _, group := range conf.Commands {
					cmds = append(cmds, sardis.CommandConf{
						Name:    group.Name,
						Command: fmt.Sprintln("sardis dmenu", group.Name),
					})
				}
			}

			opts := make([]string, 0, len(cmds))

			for _, cmd := range cmds {
				opts = append(opts, cmd.Name)
			}

			cmd, err := godmenu.RunDMenu(ctx, godmenu.Options{
				Selections: opts,
			})

			if err != nil {
				return err
			}

			return runConfiguredCommand(ctx, conf, []string{cmd})
		},
	}
}

type bufCloser struct {
	bytes.Buffer
}

func (b bufCloser) Close() error { return nil }

func qrCode(ctx context.Context) cli.Command {
	return cli.Command{
		Name:   "qr",
		Usage:  "gets qrcode from x11 clipboard and renders it on the terminal",
		Before: requireConfig(ctx),
		Action: func(c *cli.Context) error {
			buf := &bufCloser{}

			err := jasper.Context(ctx).CreateCommand(ctx).
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
