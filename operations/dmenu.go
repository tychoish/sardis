package operations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cheynewallace/tabby"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
)

type dmenuCommandType int

const (
	dmenuCommandAll dmenuCommandType = iota
	dmenuCommandGroups
)

func listMenus() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		SetUsage("prints all commands, group, and aliases.").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				table := tabby.New()
				table.AddHeader("Name", "Selections")

				for name, group := range conf.ExportCommandGroups() {
					cmds := []string{}
					for _, cmd := range group.Commands {
						if cmd.Name == "" && len(cmd.Aliases) == 0 {
							cmds = append(cmds, cmd.Command)
							for _, cg := range cmd.Commands {
								cmds = append(cmds, cg)
							}
							continue
						}
						cmds = append(cmds, cmd.Name)
						cmds = append(cmds, cmd.Aliases...)
					}
					if len(cmds) == 0 {
						grip.Debugf("skipping empty command group %q", name)
						continue
					}
					idx := -1
					for chunk := range slices.Chunk(cmds, 4) {
						idx++
						if idx == 0 {
							table.AddLine(name, strings.Join(chunk, "; "))
						} else {
							table.AddLine("", strings.Join(chunk, "; "))
						}
					}
					table.AddLine("", "")
				}

				for _, m := range conf.Menus {
					if len(m.Selections) == 0 {
						grip.Debugf("skipping empty menu %q", m.Name)
						continue
					}

					idx := -1
					for chunk := range slices.Chunk(m.Selections, 4) {
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

func DMenu() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("dmenu").
		Subcommanders(
			dmenuCommand(dmenuCommandAll).SetName("all").SetUsage("select a command from all configured commands"),
			dmenuCommand(dmenuCommandGroups).SetName("groups").SetUsage("use nested menu, starting with command groups"),
			listMenus(),
		).
		Flags(cmdr.FlagBuilder("").
			SetName(commandFlagName, "c").
			SetUsage("specify a default flag name").
			Flag()).
		With(cmdr.SpecBuilder(ResolveConfiguration).SetMiddleware(sardis.WithConfiguration).Add).
		Middleware(sardis.WithDesktopNotify).
		With(StringSpecBuilder(commandFlagName, ft.Ptr("all")).SetAction(runDmenuOperation).Add)
}

func dmenuCommand(kind dmenuCommandType) *cmdr.Commander {
	return cmdr.MakeCommander().With(cmdr.SpecBuilder(ResolveConfiguration).
		SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			switch kind {
			case dmenuCommandAll:
				return dmenuForCommands(ctx, conf, conf.ExportAllCommands())
			case dmenuCommandGroups:
				return dmenuGroupSelector(ctx, conf)
			default:
				panic(fmt.Sprintf("undefined command kind %d", kind))
			}
		}).Add)
}

func runDmenuOperation(ctx context.Context, name string) error {
	conf := sardis.AppConfiguration(ctx)

	cmdGrp := conf.ExportCommandGroups()

	if group, ok := cmdGrp[name]; ok {
		return dmenuForCommands(ctx, conf, group.Commands)
	}

	cmds, err := getcmds(conf.ExportAllCommands(), []string{name})
	if err != nil {
		return err
	}

	return dmenuForCommands(ctx, conf, cmds)
}

func dmenuGroupSelector(ctx context.Context, conf *sardis.Configuration) error {
	cmd, err := godmenu.Run(ctx,
		godmenu.ExtendSelections(conf.ExportGroupNames()),
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

	return dmenuForCommands(ctx, conf, conf.ExportCommandGroups().Get(cmd).Commands)
}

func dmenuForCommands(ctx context.Context, conf *sardis.Configuration, cmds []sardis.CommandConf) error {
	if len(cmds) == 0 {
		return errors.New("no selection")
	}

	seen := dt.Set[string]{}

	seen.AppendStream(fun.MakeConverter(func(cmd *sardis.CommandConf) string { return cmd.Name }).Stream(dt.SlicePtrs(cmds).Stream()))

	cmd, err := godmenu.Run(ctx,
		godmenu.ExtendSelections(fun.NewGenerator(seen.Stream().Slice).Force().Resolve()),
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

	ops, err := getcmds(cmds, strings.Fields(strings.TrimSpace(cmd)))
	if err != nil {
		return err
	}

	return runConfiguredCommand(ctx, ops)
}
