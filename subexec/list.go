package subexec

import (
	"fmt"
	"maps"
	"slices"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/sardis/util"
)

type CommandListStage struct {
	Selections []string
	Prefixed   []string
	Commands   []Command
	Prefix     string
	NextLabel  string
}

func (cls *CommandListStage) SelectionAt(idx int) string { return cls.Selections[idx] }
func (cls *CommandListStage) Resolve(idxs []int) []string {
	out := make([]string, 0, len(idxs))
	for _, v := range idxs {
		if len(cls.Prefixed) != 0 {
			out = append(out, cls.Prefixed[v])
		} else {
			out = append(out, cls.Selections[v])
		}
	}
	return out
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

func (conf *Configuration) ResolveCommands(args []string) (*CommandListStage, error) {
	output := CommandListStage{
		NextLabel: "sardis", // default
	}

	switch len(args) {
	case 0:
		cmds := conf.ExportAllCommands()
		output.Selections = irt.Collect(
			irt.Convert(
				irt.Slice(cmds),
				func(cmd Command) string { return cmd.NamePrime() },
			),
		)
		return &output, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			return conf.ResolveCommands(nil)
		case "groups", "group", "g":
			output.Selections = slices.Collect(maps.Keys(conf.ExportCommandGroups()))
			output.NextLabel = "groups"
			return &output, nil
		default:
			groupMap := conf.ExportCommandGroups()

			if gr, ok := groupMap[selection]; ok {
				output.NextLabel = selection
				output.Prefix = selection

				irt.Apply(
					irt.Slice(gr.Commands),
					func(c Command) {
						np := c.NamePrime()
						output.Selections = append(output.Selections, np)
						output.Prefixed = append(output.Prefixed, util.DotJoin(selection, np))
					},
				)

				return &output, nil
			}
			var err error
			if output.Commands, err = FilterCommands(conf.ExportAllCommands(), args); err != nil {
				return nil, err
			}

			return &output, nil
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
				ops := dt.MakeSet(irt.Slice(args))
				if ops.Len() != len(args) {
					return nil, fmt.Errorf("invalid list of groups %d vs %d %s", ops.Len(), len(args), args)
				}

				output.Selections = slices.Collect(slices.Values(args))
				output.NextLabel = "groups"
				return &output, nil
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
