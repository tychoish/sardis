package operations

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
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
		func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			return runConfiguredCommand(ctx, args.conf, args.ops)
		})
}

func runConfiguredCommand(ctx context.Context, conf *sardis.Configuration, ops []string) (err error) {
	// TODO avoid re-rendering this
	cmds := conf.ExportAllCommands()

	notify := sardis.DesktopNotify(ctx)

	for idx, name := range ops {
		cmd, ok := cmds[name]
		if !ok {
			return fmt.Errorf("command name %q is not defined", name)
		}
		err = jasper.Context(ctx).CreateCommand(ctx).
			Directory(cmd.Directory).
			Environment(cmd.Environment).
			AddEnv(sardis.SSHAgentSocketEnvVar, conf.SSHAgentSocket()).
			AddEnv("SARDIS_LOG_QUIET_STDOUT", "true").
			AddEnv("ALACRITTY_SOCKET", conf.AlacrittySocket()).
			ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(ops))).
			SetOutputSender(level.Info, grip.Sender()).
			SetErrorSender(level.Error, grip.Sender()).
			Background(cmd.Background).
			Append(cmd.Command).
			Append(cmd.Commands...).
			Prerequisite(func() bool {
				grip.Info(message.Fields{
					"op":   name,
					"dir":  cmd.Directory,
					"cmd":  cmd.Command,
					"cmds": cmd.Commands,
					"num":  idx + 1,
					"len":  len(ops),
				})
				return true
			}).
			PostHook(func(err error) error {
				if err != nil {
					notify.Error(message.WrapError(err, name))
					grip.Critical(err)
					return err
				}
				notify.Noticeln(name, "completed")
				return nil
			}).Run(ctx)
		if err != nil {
			break
		}
	}
	return
}

func listCommands() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		Aliases("ls").
		SetUsage("return a list of defined commands").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				homedir := util.GetHomedir()

				table := tabby.New()
				table.AddHeader("Name", "Group", "Command", "Directory")
				for _, group := range conf.Commands {
					for _, cmd := range group.Commands {
						if cmd.Directory == homedir {
							cmd.Directory = ""
						}

						grps := append([]string{group.Name}, group.Aliases...)
						if group.Name == "run" && !slices.Contains(grps, "*") {
							grps = append(grps, "*")
						}

						nms := strings.Join(append([]string{cmd.Name}, cmd.Aliases...), ", ")
						cmds := append([]string{cmd.Command}, cmd.Commands...)

						table.AddLine(
							nms,                      // names
							strings.Join(grps, ","),  // group
							strings.Join(cmds, "; "), //commands
							cmd.Directory,            // dir
						)
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
)

func dmenuListCmds(kind dmenuListCommandTypes) *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("dmenu").
		SetUsage("return a list of defined commands").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				var cmds map[string]sardis.CommandConf

				switch kind {
				case dmenuListCommandAll:
					cmds = conf.ExportAllCommands()
				case dmenuListCommandGroup:
					for _, group := range conf.Commands {
						cmds[group.Name] = sardis.CommandConf{
							Name:    group.Name,
							Command: fmt.Sprintln("sardis dmenu", group.Name),
						}
						for _, alias := range group.Aliases {
							cmds[alias] = sardis.CommandConf{
								Name:    group.Name,
								Command: fmt.Sprintln("sardis dmenu", group.Name),
							}
						}
					}

				}

				opts := make([]string, 0, len(cmds))
				seen := &dt.Set[string]{}

				for key := range cmds {
					if seen.Check(key) || key == "" {
						continue
					}
					seen.Add(key)
					opts = append(opts, key)
				}

				sort.Strings(opts)

				cmd, err := godmenu.RunDMenu(ctx, godmenu.Options{
					Selections: opts,
					DMenu:      conf.Settings.DMenu,
				})
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return err
				}

				return runConfiguredCommand(ctx, conf, []string{cmd})
			}).Add)
}

type bufCloser struct{ bytes.Buffer }

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
