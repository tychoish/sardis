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
			options, cmds, err := WriteCommandList(ctx, &args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if cmds != nil {
				return runCommands(ctx, cmds)
			}

			buf := bufio.NewWriter(os.Stdout)
			for _, opt := range options {
				_, _ = buf.WriteString(opt)
				_ = buf.WriteByte('\n')
			}

			return buf.Flush()
		},
	)
}

func WriteCommandList(ctx context.Context, conf *subexec.Configuration, args []string) ([]string, []subexec.Command, error) {
	groupMap := conf.ExportCommandGroups()
	cmds := conf.ExportAllCommands()
	options := []string{}

	switch len(args) {
	case 0:
		cmds.ReadAll(func(c subexec.Command) {
			options = append(options, c.NamePrime())
		})
		return options, nil, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			cmds.ReadAll(func(c subexec.Command) {
				options = append(options, c.NamePrime())
			})
			return options, nil, nil
		case "groups", "group", "g":
			groupMap.Keys().ReadAll(func(name string) {
				options = append(options, name)
			})
			return options, nil, nil
		default:
			if gr, ok := groupMap[selection]; ok {
				gr.Commands.ReadAll(func(c subexec.Command) {
					options = append(options, c.NamePrime())
				})
				return options, nil, nil
			}
			cmds, err := getcmds(cmds, args)
			if err != nil {
				return nil, nil, err
			}

			return nil, cmds, nil
		}
	default:
		switch args[0] {
		case "all", "a", "groups", "group", "g":
			return nil, nil, fmt.Errorf("cannot use keyword %q in context of a multi-command selection %s", args[0], args)
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
				return nil, nil, fmt.Errorf("ambiguous operation, cannot mix groups %s and commands %s", groups, missing)
			case len(groups) > 0:
				ops := dt.NewSetFromSlice(args)
				if err := groupMap.Keys().Filter(ops.Check).ReadAll(func(name string) {
					options = append(options, name)
				}).Run(ctx); err != nil {
					return nil, nil, err
				}

				return options, nil, nil
			case len(missing) > 0:
				cmds, err := getcmds(cmds, args)
				if err != nil {
					return nil, nil, err
				}
				return nil, cmds, nil
			default:
				panic("unreachable")
			}
		}
	}

}
