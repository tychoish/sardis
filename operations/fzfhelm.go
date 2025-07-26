package operations

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
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

			switch len(args.arg) {
			case 0:
				cmds.ReadAll(func(c subexec.Command) {
					_, _ = buf.WriteString(c.NamePrime())
					_ = buf.WriteByte('\n')
				})
				return buf.Flush()
			case 1:
				selection := args.arg[0]
				switch selection {
				case "all", "a":
					cmds.ReadAll(func(c subexec.Command) {
						_, _ = buf.WriteString(c.NamePrime())
						_ = buf.WriteByte('\n')
					})
					return buf.Flush()
				case "groups", "group", "g":
					groupMap.Keys().ReadAll(func(name string) {
						_, _ = buf.WriteString(name)
						_ = buf.WriteByte('\n')
					})
					return buf.Flush()
				default:
					if gr, ok := groupMap[selection]; ok {
						gr.Commands.ReadAll(func(c subexec.Command) {
							_, _ = buf.WriteString(c.NamePrime())
							_ = buf.WriteByte('\n')
						})
						return buf.Flush()
					}

					return runMatchingCommands(ctx, cmds, args.arg)
				}
			default:
				switch args.arg[0] {
				case "all", "a", "groups", "group", "g":
					return fmt.Errorf("cannot use keyword %q in context of a multi-command selection %s", args.arg[0], args.arg)
				default:
					var missing []string
					var groups []string
					for _, item := range args.arg {
						if _, ok := groupMap[item]; ok {
							groups = append(groups, item)
						}
						missing = append(missing, item)
					}
					switch {
					case len(missing) > 0 && len(groups) > 0:
						return fmt.Errorf("ambiguous operation, cannot mix groups %s and commands %s", groups, missing)
					case len(groups) > 0:
						ops := dt.NewSetFromSlice(args.arg)
						if err := groupMap.Keys().Filter(ops.Check).ReadAll(func(name string) {
							_, _ = buf.WriteString(name)
							_ = buf.WriteByte('\n')
						}).Run(ctx); err != nil {
							return err
						}

						return buf.Flush()
					case len(missing) > 0:
						return runMatchingCommands(ctx, cmds, args.arg)
					default:
						panic("unreachable")
					}
				}
			}
		},
	)
}
