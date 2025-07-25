package operations

import (
	"bufio"
	"context"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
)

func SearchMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("cmds").
		SetUsage("list or run a command").
		Aliases("c", "m").
		Subcommanders(
		// dmenuCommand(dmenuCommandAll).SetName("all").SetUsage("select a command from all configured commands"),
		// dmenuCommand(dmenuCommandGroups).SetName("groups").SetUsage("use nested menu, starting with command groups"),
		// listMenus(),
		),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			groupMap := args.conf.Operations.ExportCommandGroups()
			cmds := args.conf.Operations.ExportAllCommands()
			buf := bufio.NewWriter(os.Stdout)

			notify := sardis.DesktopNotify(ctx)

			switch len(args.arg) {
			case 0:
				grip.Info("check on all of the things")
				cmds.ReadAll(func(c subexec.Command) {
					buf.WriteString(c.NamePrime())
					buf.WriteByte('\n')
				})
				return buf.Flush()
			case 1:
				selection := args.arg[0]
				switch selection {
				case "all", "a":
					grip.Info("loop back to all")
					cmds.ReadAll(func(c subexec.Command) {
						buf.WriteString(c.NamePrime())
						buf.WriteByte('\n')
					})
					return buf.Flush()
				case "groups", "group", "g":
					groupMap.Keys().ReadAll(func(name string) {
						buf.WriteString(name)
						buf.WriteByte('\n')
					})
					return buf.Flush()
				default:
					if gr, ok := groupMap[selection]; ok {
						notify.Errorf("UNIMPLEMENTED: select from group %q", gr.Name)
						return ers.ErrNotImplemented
					}

					return runMatchingCommands(ctx, cmds, args.arg)
				}
			// groups, then prefix, then everything (running)
			default:
				notify.Errorf("UNIMPLEMENTED: %s", args.arg)
				return runMatchingCommands(ctx, cmds, args.arg)
				// if all items are commands then run them, but we can't drill into multiple at once (probably?)
			}
		},
	)
}
