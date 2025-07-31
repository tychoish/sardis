package operations

import (
	"bufio"
	"context"
	"fmt"
	"iter"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/cheynewallace/tabby"
	fzf "github.com/koki-develop/go-fzf"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

const commandFlagName string = "command"

func RunCommand() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("run").
		Aliases("r").
		SetUsage("runs a predefined command").
		Subcommanders(
			listCommands(),
		),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			cmds, err := subexec.FilterCommands(args.conf.Operations.ExportAllCommands(), args.arg)
			if err != nil {
				return ers.Wrapf(err, "resolving commands %s", args.arg)
			}

			return subexec.RunCommands(ctx, cmds)
		})
}

func SearchMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("cmd").
		SetUsage("list or run a command").
		Aliases("c", "m", "cmds").
		Subcommanders(
			listCommands(),
			fuzzy(),
		),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			stage, err := subexec.WriteCommandList(&args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if stage.Commands != nil {
				return subexec.RunCommands(ctx, stage.Commands)
			}

			var ops iter.Seq[string]
			if len(stage.Prefixed) != 0 {
				ops = slices.Values(stage.Prefixed)
			} else {
				ops = slices.Values(stage.Selections)
			}

			buf := bufio.NewWriter(os.Stdout)

			for op := range ops {
				ft.Ignore(ft.Must(fmt.Fprintln(buf, op)))
			}

			return buf.Flush()
		},
	)
}

func DMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("dmenu").
		SetUsage("unless running a subcommand, launches a menu for specific group specific group, or attmepts to run a command directly.").
		Subcommanders(
			listCommands(),
		),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg
			var selected string

			for {
				stage, err := subexec.WriteCommandList(&args.conf.Operations, op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					return subexec.RunCommands(ctx, stage.Commands)
				case stage.Selections != nil:
					selected, err = godmenu.Run(ctx,
						godmenu.SetSelections(op),
						godmenu.WithFlags(ft.Ptr(args.conf.Settings.DMenuFlags)),
						godmenu.Prompt(fmt.Sprintf("%s ==>>", ft.Default(stage.NextLabel, "sardis"))),
						godmenu.MenuLines(min(len(stage.Selections), 16)),
					)

					switch {
					case err == nil && len(stage.Prefixed) != 0:
						selected = stage.Prefixed[0]
					case err == nil && len(stage.Prefixed) == 0:
						selected = stage.Selections[0]
					case ers.Is(err, godmenu.ErrSelectionMissing):
						return nil
					case err != nil:
						return err
					}

					op = []string{selected}
				default:
					// this should be impossible
					return ers.Error("unexpect outcome")
				}
			}
		})
}

func fuzzy() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fuzzy").
			Aliases("fuzz", "fzf", "f", "ff"),
		"name",
		func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg

			ff, err := fzf.New(
				fzf.WithPrompt(fmt.Sprintf("%s.%s ==> ", util.GetHostname(), sardis.ApplicationName)),
				fzf.WithNoLimit(true),
				fzf.WithCaseSensitive(false),
			)
			if err != nil {
				return err
			}

			opr := GetOperationRuntime(ctx)
			for {
				stage, err := subexec.WriteCommandList(&args.conf.Operations, op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					startedAt := time.Now()
					err := subexec.RunCommands(ctx, stage.Commands)
					ranFor := time.Since(startedAt)

					if opr.ShouldBlock {
						<-ctx.Done()
					}

					waited := time.Since(startedAt) - ranFor

					grip.Notice(message.BuildPair().
						Pair("op", "cmd.fuzzy").
						Pair("state", "COMPLETED").
						Pair("err", err != nil).
						Pair("runtime", ranFor).
						Pair("waited", waited).
						Pair("commands", strings.Join(stage.CommandNames(), ", ")))

					return err
				case stage.Selections != nil:
					idxs, err := ff.Find(
						stage.Selections,
						func(idx int) string {
							return stage.Selections[idx]
						})
					if err != nil {
						return err
					}

					op = make([]string, 0, len(idxs))
					for _, v := range idxs {
						if len(stage.Prefixed) != 0 {
							op = append(op, stage.Prefixed[v])
						} else {
							op = append(op, stage.Selections[v])
						}
					}
				default:
					// this should be impossible
					return ers.Error("unexpect outcome")
				}
			}
		})
}

func listCommands() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("list").
			Aliases("ls", "l").
			Subcommanders(
				listCommandsWithInfo(),
				listCommandsPlain(),
			).
			SetUsage("prints all commands, group, and aliases."),
		"group", func(ctx context.Context, args *withConf[[]string]) error {
			conf := args.conf
			set := dt.NewSetFromSlice(args.arg)

			groups := conf.Operations.ExportCommandGroups()
			groupSet := &dt.Set[string]{}
			groupSet.AppendStream(groups.Keys())

			table := tabby.New()
			table.AddHeader("Category", "Group", "Prefix", "Name")

			for name, group := range groups {
				if set.Len() == 0 || set.Check(name) {
					for idx, cc := range group.Commands {
						if idx == 0 {
							table.AddLine(group.Category, group.Name, group.CmdNamePrefix, cc.Name)
						} else {
							table.AddLine("", "", "", cc.Name)
						}
					}
					table.AddLine("", "", "", "")
				}
			}

			table.Print()
			return nil
		})
}

func listCommandsPlain() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("plain").
		Aliases("p", "pl").
		SetUsage("return a simple printed list of commands.").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				stage, err := subexec.WriteCommandList(&conf.Operations, nil)

				switch {
				case err != nil:
					return err
				case len(stage.Selections) != 0:
					return fmt.Errorf("found invalid selections %s", stage.Selections)
				case len(stage.Commands) == 0:
					return ers.Error("no commands defined")
				}

				var ops iter.Seq[string]
				if len(stage.Prefixed) != 0 {
					ops = slices.Values(stage.Prefixed)
				} else {
					ops = slices.Values(stage.Selections)
				}

				buf := bufio.NewWriter(os.Stdout)

				for op := range ops {
					ft.Ignore(ft.Must(fmt.Fprintln(buf, op)))
				}

				return buf.Flush()
			}).Add)
}

func listCommandsWithInfo() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("info").
		Aliases("extra", "x", "shell").
		SetUsage("return a list of defined commands").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				homedir := util.GetHomeDir()

				table := tabby.New()
				table.AddHeader("Name", "Command", "Directory")
				for _, cmd := range conf.Operations.ExportAllCommands() {
					if cmd.Directory == homedir {
						cmd.Directory = ""
					}

					cmds := append([]string{cmd.Command}, cmd.Commands...)

					for idx := range cmds {
						if maxLen := 48; len(cmds[idx]) > maxLen {
							cmds[idx] = fmt.Sprintf("<%s...>", cmds[idx][:maxLen])
						}
					}

					for idx, chunk := range cmds {
						if idx == 0 {
							table.AddLine(
								cmd.Name,                               // name
								chunk,                                  // command
								util.TryCollapseHomeDir(cmd.Directory), // dir
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

				table.Print()

				return nil
			}).Add)
}
