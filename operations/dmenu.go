package operations

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cheynewallace/tabby"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
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
		commandFlagName, func(ctx context.Context, args *withConf[string]) error {
			name := args.arg
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
	items := conf.Operations.ExportGroupNames()
	name, err := godmenu.Run(ctx,
		godmenu.Items(items...),
		godmenu.WithFlags(ft.Ptr(conf.Settings.DMenuFlags)),
		godmenu.Prompt("groups ==>>"),
		godmenu.MenuLines(min(len(items), 16)),
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

	items := group.Selectors()
	cmd, err := godmenu.Run(ctx,
		godmenu.Items(items...),
		godmenu.WithFlags(ft.Ptr(conf.Settings.DMenuFlags)),
		godmenu.Prompt(fmt.Sprint(group.Name, " ==>>")),
		godmenu.MenuLines(min(len(items), 16)),
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
