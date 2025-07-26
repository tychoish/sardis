package operations

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
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
			output, cmds, err := WriteCommandList(ctx, &args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if len(cmds) > 1 {
				runCommands(ctx, cmds)
			}

			buf := bufio.NewWriter(os.Stdout)

			return erc.Join(ft.IgnoreFirst(fmt.Fprint(buf, output)), buf.Flush())
		},
	)
}

func WriteCommandList(ctx context.Context, conf *subexec.Configuration, args []string) (string, []subexec.Command, error) {
	groupMap := conf.ExportCommandGroups()
	cmds := conf.ExportAllCommands()
	buf := &bytes.Buffer{}

	switch len(args) {
	case 0:
		cmds.ReadAll(func(c subexec.Command) {
			_, _ = buf.WriteString(c.NamePrime())
			_ = buf.WriteByte('\n')
		})
		return buf.String(), nil, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			cmds.ReadAll(func(c subexec.Command) {
				_, _ = buf.WriteString(c.NamePrime())
				_ = buf.WriteByte('\n')
			})
			return buf.String(), nil, nil
		case "groups", "group", "g":
			groupMap.Keys().ReadAll(func(name string) {
				_, _ = buf.WriteString(name)
				_ = buf.WriteByte('\n')
			})
			return buf.String(), nil, nil
		default:
			if gr, ok := groupMap[selection]; ok {
				gr.Commands.ReadAll(func(c subexec.Command) {
					_, _ = buf.WriteString(c.NamePrime())
					_ = buf.WriteByte('\n')
				})
				return buf.String(), nil, nil
			}
			cmds, err := getcmds(cmds, args)
			return "", cmds, err
		}
	default:
		switch args[0] {
		case "all", "a", "groups", "group", "g":
			return "", nil, fmt.Errorf("cannot use keyword %q in context of a multi-command selection %s", args[0], args)
		default:
			var missing []string
			var groups []string
			for _, item := range args {
				if _, ok := groupMap[item]; ok {
					groups = append(groups, item)
				}
				missing = append(missing, item)
			}
			switch {
			case len(missing) > 0 && len(groups) > 0:
				return "", nil, fmt.Errorf("ambiguous operation, cannot mix groups %s and commands %s", groups, missing)
			case len(groups) > 0:
				ops := dt.NewSetFromSlice(args)
				if err := groupMap.Keys().Filter(ops.Check).ReadAll(func(name string) {
					_, _ = buf.WriteString(name)
					_ = buf.WriteByte('\n')
				}).Run(ctx); err != nil {
					return "", nil, err
				}

				return buf.String(), nil, nil
			case len(missing) > 0:
				cmds, err := getcmds(cmds, args)
				return "", cmds, err
			default:
				panic("unreachable")
			}
		}
	}

}
