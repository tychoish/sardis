package operations

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
)

func SearchMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("cmds").
		SetUsage("list or run a command").
		Aliases("c", "m").
		Subcommanders(listMenus()),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			stage, err := WriteCommandList(ctx, &args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if stage.Commands != nil {
				return runCommands(ctx, stage.Commands)
			}

			buf := bufio.NewWriter(os.Stdout)
			for _, opt := range stage.Selections {
				if stage.Prefix != "" {
					fmt.Fprintf(buf, "%s.%s", stage.Prefix, opt)
				} else {
					_, _ = buf.WriteString(opt)
				}
				_ = buf.WriteByte('\n')
			}

			return buf.Flush()
		},
	)
}

type CommandListStage struct {
	NextLabel  string
	Prefix     string
	Selections []string
	Commands   []subexec.Command
}

func WriteCommandList(ctx context.Context, conf *subexec.Configuration, args []string) (*CommandListStage, error) {
	var options []string

	switch len(args) {
	case 0:
		cmds := conf.ExportAllCommands()
		options = make([]string, 0, len(cmds))
		cmds.ReadAll(func(c subexec.Command) {
			options = append(options, c.NamePrime())
		})
		return &CommandListStage{NextLabel: sardis.ApplicationName, Selections: options}, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			return WriteCommandList(ctx, conf, nil)
		case "groups", "group", "g":
			conf.ExportCommandGroups().Keys().ReadAll(func(name string) {
				options = append(options, name)
			}).Run(ctx)
			return &CommandListStage{NextLabel: "groups", Selections: options}, nil
		default:
			groupMap := conf.ExportCommandGroups()

			if gr, ok := groupMap[selection]; ok {
				gr.Commands.ReadAll(func(c subexec.Command) {
					options = append(options, c.NamePrime())
				})
				return &CommandListStage{NextLabel: selection, Selections: options, Prefix: selection}, nil
			}

			cmds, err := getcmds(conf.ExportAllCommands(), args)
			if err != nil {
				return nil, err
			}

			return &CommandListStage{Commands: cmds}, nil
		}
	default:
		switch args[0] {
		case "all", "a", "groups", "group", "g":
			return nil, fmt.Errorf("cannot use keyword %q in context of a multi-command selection %s", args[0], args)
		default:
			groupMap := conf.ExportCommandGroups()

			var missing []string
			var groups []string
			for _, item := range args {
				if _, ok := groupMap[item]; ok {
					groups = append(groups, item)
				} else {
					missing = append(missing, item)
				}
			}

			switch {
			case len(missing) > 0 && len(groups) > 0:
				return nil, fmt.Errorf("ambiguous operation, cannot mix groups %s and commands %s", groups, missing)
			case len(groups) > 0:
				ops := dt.NewSetFromSlice(args)
				if err := groupMap.Keys().Filter(ops.Check).ReadAll(func(name string) {
					options = append(options, name)
				}).Run(ctx); err != nil {
					return nil, err
				}
				return &CommandListStage{NextLabel: "groups", Selections: options}, nil
			case len(missing) > 0:
				cmds, err := getcmds(conf.ExportAllCommands(), args)
				if err != nil {
					return nil, err
				}
				return &CommandListStage{Commands: cmds}, nil
			default:
				panic("unreachable")
			}
		}
	}
}
