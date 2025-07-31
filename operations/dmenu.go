package operations

import (
	"context"
	"fmt"

	"github.com/cheynewallace/tabby"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/godmenu"
)

func DMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("dmenu").
		SetUsage("unless running a subcommand, launches a menu for specific group specific group, or attmepts to run a command directly.").
		Subcommanders(listMenus()),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg
			var selected string

			for {
				stage, err := subexec.WriteCommandList(ctx, &args.conf.Operations, op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					return runCommands(ctx, stage.Commands)
				case stage.Selections != nil:
					selected, err = godmenu.Run(ctx,
						godmenu.SetSelections(stage.Selections),
						godmenu.WithFlags(ft.Ptr(args.conf.Settings.DMenuFlags)),
						godmenu.Prompt(fmt.Sprintf("%s ==>>", ft.Default(stage.NextLabel, "sardis"))),
						godmenu.MenuLines(min(len(stage.Selections), 16)),
					)
					if err != nil {
						if ers.Is(err, godmenu.ErrSelectionMissing) {
							return nil
						}
						return err
					}
					if stage.Prefix != "" {
						selected = fmt.Sprintf("%s.%s", stage.Prefix, selected)
					}
					op = []string{selected}
				default:
					// this should be impossible
					return ers.Error("unexpect outcome")
				}
			}
		})
}

func listMenus() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("list").
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
				if set.Len() > 0 && !set.Check(name) {
					continue
				}

				for idx, cc := range group.Commands {
					if idx == 0 {
						table.AddLine(group.Category, group.Name, group.CmdNamePrefix, cc.Name)
					} else {
						table.AddLine("", "", "", cc.Name)
					}
				}
				table.AddLine("", "", "", "")
			}

			table.Print()
			return nil
		})
}
