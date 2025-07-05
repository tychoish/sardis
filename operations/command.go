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
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
	sutil "github.com/tychoish/sardis/util"
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
			cmds, err := getcmds(args.conf, args.conf.ExportAllCommands(), args.ops)
			if err != nil {
				return err
			}
			return runConfiguredCommand(ctx, args.conf, cmds)
		})
}

func getcmds(conf *sardis.Configuration, cmds []sardis.CommandConf, args []string) ([]sardis.CommandConf, error) {
	// TODO make this a method on conf.
	out := make([]sardis.CommandConf, 0, len(args))

	ops := dt.NewSetFromSlice(args)
	seen := make([]string, 0, len(args))

	for idx := range cmds {
		name := cmds[idx].Name
		// if the name of the ops matches one we're looking
		// for or if the name of the command starts with the
		// group name "run." check the inner name.
		if ops.Check(name) || (strings.HasPrefix(name, "run.") && ops.Check(name[4:])) {
			out = append(out, cmds[idx])
			seen = append(seen, name)
		}
	}

	// if we didn't find all that we were looking for?
	if ops.Len() != len(out) {
		return nil, fmt.Errorf("found %d ops, of %d, ops %q; found %q ",
			len(out), ops.Len(),
			// TODO we should be able to get slices from sets without panic
			strings.Join(fun.NewGenerator(ops.Stream().Slice).Force().Resolve(), ", "),
			strings.Join(seen, ", "),
		)
	}

	return out, nil
}

func runConfiguredCommand(ctx context.Context, conf *sardis.Configuration, cmds []sardis.CommandConf) (err error) {
	jpm := jasper.Context(ctx)
	for idx, cmd := range cmds {
		name := cmd.Name
		err = jpm.CreateCommand(ctx).
			Directory(cmd.Directory).
			Environment(cmd.Environment).
			AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
			AddEnv(sardis.EnvVarAlacrittySocket, conf.AlacrittySocket()).
			AddEnv(sardis.EnvVarSSHAgentSocket, conf.SSHAgentSocket()).
			ID(fmt.Sprintf("%s.%d/%d", name, idx+1, len(cmds))).
			SetOutputSender(level.Info, grip.Sender()).
			SetErrorSender(level.Error, grip.Sender()).
			Background(ft.Ref(cmd.Background)).
			Append(cmd.Command).
			Append(cmd.Commands...).
			Prerequisite(func() bool {
				grip.Info(message.Fields{
					"op":   name,
					"dir":  cmd.Directory,
					"cmd":  cmd.Command,
					"cmds": cmd.Commands,
					"num":  idx + 1,
					"len":  len(cmds),
				})
				return true
			}).
			PostHook(func(err error) error {
				notify := sardis.DesktopNotify(ctx)
				if err != nil {
					notify.Error(message.WrapError(err, name))
					grip.Critical(err)
					return err
				}
				notify.Notice(message.Whenln(ft.Ref(cmd.Notify), name, "completed"))
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
				table.AddHeader("Group", "Name", "Command", "Directory")

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
						for idx := range cmds {
							if maxLen := 52; len(cmds[idx]) > maxLen {
								cmds[idx] = fmt.Sprintf("<%s...>", cmds[idx][:maxLen])
							}
						}
						for idx, chunk := range cmds {
							if idx == 0 {
								table.AddLine(
									strings.Join(grps, ","),                 // group
									nms,                                     // names
									chunk,                                   // command
									sutil.TryCollapseHomedir(cmd.Directory), // dir
								)
							} else {
								table.AddLine(
									"",    // group
									"",    // names
									chunk, // command
									"",    // dir
								)

							}
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
					cmds = conf.ExportAllCommands()
				case dmenuListCommandGroup:
					for _, group := range conf.Commands {
						cmds = append(cmds, sardis.CommandConf{
							Name:    group.Name,
							Command: fmt.Sprintln("sardis dmenu", group.Name),
						})

						for _, alias := range group.Aliases {
							cmds = append(cmds, sardis.CommandConf{
								Name:    alias,
								Command: fmt.Sprintln("sardis dmenu", group.Name),
							})
						}
					}

				}

				opts := make([]string, 0, len(cmds))
				seen := dt.Set[string]{}

				// TODO could rebuild the map here,
				// and pass to the runner

				for _, cmd := range cmds {
					if seen.Check(cmd.Name) {
						continue
					}
					seen.Add(cmd.Name)
					opts = append(opts, cmd.Name)

				}

				sort.Strings(opts)

				cmd, err := godmenu.Run(ctx,
					godmenu.ExtendSelections(opts),
					godmenu.WithFlags(conf.Settings.DMenu),
					godmenu.Sorted(),
				)
				switch {
				case err == nil:
					break
				case ers.Is(err, godmenu.ErrSelectionMissing):
					return nil
				default:
					return err
				}

				ops, err := getcmds(conf, cmds, []string{cmd})
				if err != nil {
					return err
				}

				return runConfiguredCommand(ctx, conf, ops)
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
