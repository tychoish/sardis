package subexec

import (
	"context"
	"fmt"
	"slices"

	"github.com/tychoish/fun/dt"
)

type CommandListStage struct {
	NextLabel  string
	Prefix     string
	Selections []string
	Commands   []Command
}

func (cls CommandListStage) CommandNames() []string {
	if len(cls.Commands) == 0 {
		return nil
	}
	out := make([]string, 0, len(cls.Commands))
	for cmd := range slices.Values(cls.Commands) {
		out = append(out, cmd.FQN())
	}
	return out
}

func WriteCommandList(ctx context.Context, conf *Configuration, args []string) (*CommandListStage, error) {
	var options []string

	switch len(args) {
	case 0:
		cmds := conf.ExportAllCommands()
		options = make([]string, 0, len(cmds))
		cmds.ReadAll(func(c Command) {
			options = append(options, c.NamePrime())
		})
		return &CommandListStage{NextLabel: "sardis", Selections: options}, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			return WriteCommandList(ctx, conf, nil)
		case "groups", "group", "g":
			conf.ExportCommandGroups().Keys().ReadAll(func(name string) {
				options = append(options, name)
			}).Ignore().Run(ctx)
			return &CommandListStage{NextLabel: "groups", Selections: options}, nil
		default:
			groupMap := conf.ExportCommandGroups()

			if gr, ok := groupMap[selection]; ok {
				gr.Commands.ReadAll(func(c Command) {
					options = append(options, c.NamePrime())
				})
				return &CommandListStage{NextLabel: selection, Selections: options, Prefix: selection}, nil
			}

			cmds, err := FilterCommands(conf.ExportAllCommands(), args)
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
				cmds, err := FilterCommands(conf.ExportAllCommands(), args)
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
