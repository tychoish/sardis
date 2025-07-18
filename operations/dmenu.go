package operations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cheynewallace/tabby"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
)

type dmenuCommandType int

const (
	dmenuCommandAll dmenuCommandType = iota
	dmenuCommandGroups
)

func DMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("dmenu").
		SetUsage("unless running a subcommand, launches a menu for specific group specific group, or attmepts to run a command directly.").
		Subcommanders(
			dmenuCommand(dmenuCommandAll).SetName("all").SetUsage("select a command from all configured commands"),
			dmenuCommand(dmenuCommandGroups).SetName("groups").SetUsage("use nested menu, starting with command groups"),
			listMenus(),
		),
		commandFlagName, func(ctx context.Context, args *opsCmdArgs[string]) error {
			name := args.ops
			if group, ok := args.conf.Operations.ExportCommandGroups()[name]; ok {
				return dmenuForCommands(ctx, args.conf, group)
			}

			cmds, err := getcmds(args.conf.Operations.ExportAllCommands(), []string{name})
			if err != nil {
				return err
			}

			return runConfiguredCommand(ctx, cmds)
		})
}

func dmenuCommand(kind dmenuCommandType) *cmdr.Commander {
	return cmdr.MakeCommander().With(cmdr.SpecBuilder(ResolveConfiguration).
		SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			switch kind {
			case dmenuCommandAll:
				return dmenuForCommands(ctx, conf, subexec.Group{Name: "sardis", Commands: conf.Operations.ExportAllCommands()})
			case dmenuCommandGroups:
				return dmenuForGroups(ctx, conf)
			default:
				panic(fmt.Sprintf("undefined command kind %d", kind))
			}
		}).Add)
}

func dmenuForGroups(ctx context.Context, conf *sardis.Configuration) error {
	name, err := godmenu.Run(ctx,
		godmenu.ExtendSelections(conf.Operations.ExportGroupNames()),
		godmenu.WithFlags(ft.Ptr(conf.Settings.DMenu)),
		godmenu.Sorted(),
		godmenu.Prompt("groups ==>>"),
	)
	if err != nil {
		return erc.NewFilter().Without(godmenu.ErrSelectionMissing).Apply(err)
	}

	if group, ok := conf.Operations.ExportCommandGroups()[name]; ok {
		return dmenuForCommands(ctx, conf, group)
	}

	return fmt.Errorf("selection %q not found", name)
}

func dmenuForCommands(ctx context.Context, conf *sardis.Configuration, group subexec.Group) error {
	if len(group.Commands) == 0 {
		return errors.New("no selection")
	}

	cmd, err := godmenu.Run(ctx,
		godmenu.ExtendSelections(group.Selectors()),
		godmenu.WithFlags(ft.Ptr(conf.Settings.DMenu)),
		godmenu.Sorted(),
		godmenu.Prompt(fmt.Sprint(group.Name, " ==>>")),
	)

	if err != nil {
		return erc.NewFilter().Without(godmenu.ErrSelectionMissing).Apply(err)
	}

	ops, err := getcmds(group.Commands, strings.Fields(strings.TrimSpace(cmd)))
	if err != nil {
		return err
	}

	return runConfiguredCommand(ctx, ops)
}

func listMenus() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		SetUsage("prints all commands, group, and aliases.").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				table := tabby.New()
				table.AddHeader("Name", "Selections")

				for name, group := range conf.Operations.ExportCommandGroups() {
					cmds := []string{}
					for _, cmd := range group.Commands {
						cmds = append(cmds, cmd.Name)
					}
					if len(cmds) == 0 {
						grip.Debugf("skipping empty command group %q", name)
						continue
					}
					idx := -1
					for chunk := range slices.Chunk(cmds, 3) {
						idx++
						if idx == 0 {
							table.AddLine(name, strings.Join(chunk, "; "))
						} else {
							table.AddLine("", strings.Join(chunk, "; "))
						}
					}
					table.AddLine("", "")
				}

				// TODO() move menus into commands and
				// ignore them in rendering
				for _, m := range conf.Menus {
					if len(m.Selections) == 0 {
						grip.Debugf("skipping empty menu %q", m.Name)
						continue
					}

					idx := -1
					for chunk := range slices.Chunk(m.Selections, 3) {
						idx++
						if idx == 0 {
							table.AddLine(m.Name, strings.Join(chunk, "; "))
						} else {
							table.AddLine("", strings.Join(chunk, "; "))
						}
					}
					table.AddLine("", "")
				}

				table.Print()
				return nil
			}).Add)
}
