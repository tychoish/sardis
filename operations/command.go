package operations

import (
	"bytes"
	"context"
	"fmt"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/cheynewallace/tabby"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli/v2"
)

const commandFlagName = "command"

func RunCommand() *cmdr.Commander {
	cmd := cmdr.MakeCommander().SetName("run").
		SetUsage("runs a predefined command").
		Subcommanders(
			listCommands(),
			dmenuListCmds(dmenuListCommandAll),
			qrCode(),
		)
	return addOpCommand(cmd, "command",
		func(ctx context.Context, args *opsCmdArgs) error {
			return runConfiguredCommand(ctx, args.conf, args.ops)
		})
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
				notify.Sender().SetName(name)
				notify.Notice("completed")
				return nil
			}).
			Run(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func listCommands() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		Aliases("ls").
		SetUsage("return a list of defined commands").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
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
			}).Add)
}

type dmenuListCommandTypes int

const (
	dmenuListCommandAll dmenuListCommandTypes = iota
	dmenuListCommandGroup
	dmenuListCommandRun
)

func dmenuListCmds(kind dmenuListCommandTypes) *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("dmenu").
		SetUsage("return a list of defined commands").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
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
			}).Add)
}

type bufCloser struct {
	bytes.Buffer
}

func (b bufCloser) Close() error { return nil }

func qrCode() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("qr").
		SetUsage("gets qrcode from x11 clipboard and renders it on the terminal").
		SetAction(func(ctx context.Context, _ *cli.Context) error {
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
		})
}
