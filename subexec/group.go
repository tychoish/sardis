package subexec

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/grip"
	jutil "github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/util"
)

type Group struct {
	Category       string                  `bson:"category" json:"category" yaml:"category"`
	Name           string                  `bson:"name" json:"name" yaml:"name"`
	Aliases        []string                `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory      string                  `bson:"directory" json:"directory" yaml:"directory"`
	Environment    stw.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	CmdNamePrefix  string                  `bson:"command_name_prefix" json:"command_name_prefix" yaml:"command_name_prefix"`
	Command        string                  `bson:"default_command" json:"default_command" yaml:"default_command"`
	Notify         *bool                   `bson:"notify" json:"notify" yaml:"notify"`
	Background     *bool                   `bson:"background" json:"background" yaml:"background"`
	Host           *string                 `bson:"host" json:"host" yaml:"host"`
	Commands       stw.Slice[Command]      `bson:"commands" json:"commands" yaml:"commands"`
	MenuSelections []string                `bson:"menu" json:"menu" yaml:"menu"`
	SortHint       int                     `bson:"sort_hint" json:"sort_hint" yaml:"sort_hint"`
	Synthetic      bool                    `bson:"-" json:"-" yaml:"-"`
}

func (cg *Group) ResolvedCategory() string {
	if cg.Category != "" {
		return cg.Category
	}
	return cg.Name
}
func (cg *Group) ID() string       { return util.DotJoinParts(cg.IDPath()) }
func (cg *Group) IDPath() []string { return []string{cg.Category, cg.Name, cg.CmdNamePrefix} }

func (cg *Group) NamesAtIndex(idx int) []string {
	erc.InvariantOk(idx >= 0 && idx < len(cg.Commands), "command out of bounds", cg.Name)
	ops := []string{}

	for _, grp := range append([]string{cg.Name}, cg.Aliases...) {
		ops = append(ops, util.DotJoin(cg.Category, grp, cg.Commands[idx].Name))
	}

	return ops
}

func (cg *Group) Selectors() []string {
	set := &dt.OrderedSet[string]{}

	for cmd := range slices.Values(cg.Commands) {
		set.Add(cmd.Name)
	}

	return irt.Collect(set.Iterator())
}

func (cg *Group) Validate() error {
	home := util.GetHomeDir()

	ec := &erc.Collector{}

	for _, selection := range cg.MenuSelections {
		cg.Commands = append(cg.Commands, Command{Name: selection, Command: selection})
	}

	{ // this is in braces because it's sus as hell.
		if cg.Category == "" && cg.Name != "" && cg.CmdNamePrefix != "" {
			grip.Warningf("deprecated and unnecessary mangling for %s and %s", cg.Category, cg.Name)
			cg.Category = cg.Name
			cg.Name = cg.CmdNamePrefix
			cg.CmdNamePrefix = ""
		}

		if cg.Name == "" && cg.CmdNamePrefix != "" {
			grip.Warningf("command group cat=%q prefix=%q is missing name, rotating name", cg.Category, cg.CmdNamePrefix)
			cg.Name = cg.CmdNamePrefix
			cg.CmdNamePrefix = ""
		}

	}

	ec.If(cg.Name == "", ers.Error("command group must have name"))

	for idx := range cg.Commands {
		cmd := cg.Commands[idx]
		cmd.GroupCategory = cg.Category
		cmd.GroupName = cg.Name
		cmd.Notify = util.Default(cmd.Notify, cg.Notify)
		cmd.Background = util.Default(cmd.Background, cg.Background)
		cmd.Directory = jutil.TryExpandHomedir(util.Default(cmd.Directory, home))

		ec.Whenf(cmd.Name == "", "command in group [%s](%d) must have a name", cg.Name, idx)
		ec.Whenf(cmd.Command == "" && cmd.OverrideDefault, "cannot override default without an override, in group [%s] command [%s] at index (%d)", cg.Name, cmd.Name, idx)

		if cg.Environment != nil || cmd.Environment != nil {
			env := stw.Map[string, string]{}
			if cg.Environment != nil {
				env.Extend(cg.Environment.Iterator())
			}

			if cmd.Environment != nil {
				env.Extend(cmd.Environment.Iterator())
			}

			cmd.Environment = env
		}

		if !cmd.OverrideDefault {
			cmd.Command = util.Default(cmd.Command, cg.Command)
			cmd.Command = util.Default(cmd.Command, cmd.Name)
			cmd.Command = util.Default(strings.ReplaceAll(cg.Command, "{{command}}", cmd.Command), cmd.Command)
		}

		if cc := cmd.Command; strings.Contains(cc, "{{") && strings.Contains(cc, "}}") {
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{name}}", cmd.Name)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.category}}", cg.Category)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.name}}", cg.Name)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{host}}", stw.Deref(cg.Host))
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{prefix}}", cg.CmdNamePrefix)
		}

		for idx := range cmd.Commands {
			if cc := cmd.Commands[idx]; strings.Contains(cc, "{{") && strings.Contains(cc, "}}") {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{command}}", cmd.Command)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{name}}", cmd.Name)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{host}}", stw.Deref(cg.Host))
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{group.name}}", cg.Name)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{group.category}}", cg.Category)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{prefix}}", cg.Name)
			}
		}

		cmd.Name = util.DotJoin(cg.CmdNamePrefix, cmd.Name)

		cg.Commands[idx] = cmd
	}
	cg.Command = ""
	cg.Environment = nil

	return ec.Resolve()
}

func (cg *Group) doMerge(rhv Group) bool {
	if (cg.Category == "" || rhv.Category == "") && cg.Name != rhv.Name {
		return false
	} else if cg.Category != rhv.Category {
		return false
	}

	cg.Aliases = nil
	if cg.SortHint >= rhv.SortHint {
		cg.Commands = append(cg.Commands, rhv.Commands...)
	} else {
		cg.Commands = append(rhv.Commands, cg.Commands...)
	}
	// AbsMax: return the value with the larger absolute value
	absA, absB := cg.SortHint, rhv.SortHint
	if absA < 0 {
		absA = -absA
	}
	if absB < 0 {
		absB = -absB
	}
	if absA >= absB {
		cg.SortHint = cg.SortHint
	} else {
		cg.SortHint = rhv.SortHint
	}

	return true
}

func FilterCommands(cmds stw.Slice[Command], args []string) (stw.Slice[Command], error) {
	ops := dt.MakeSet(irt.Slice(args))
	switch {
	case len(cmds) == 0:
		return nil, ers.New("cannot resolve commands without input commands")
	case len(args) == 0:
		return nil, ers.New("must specify one or more commands to resolve")
	case ops.Len() != len(args):
		return nil, fmt.Errorf("ambiguous input with %d duplicate items %s", ops.Len()-len(args), args)
	}

	seen := dt.OrderedSet[string]{}

	out := irt.Collect(irt.Keep(irt.Slice(cmds), func(cmd Command) bool {
		name := cmd.NamePrime()
		return ops.Check(name) && !seen.Add(name)
	}), 0, len(cmds))

	// if we didn't find all that we were looking for?
	if ops.Len() != len(out) {
		return nil, fmt.Errorf("found %d [%s] ops, of %d [%s] arguments",
			len(out), strings.Join(irt.Collect(seen.Iterator()), ", "),
			ops.Len(), strings.Join(irt.Collect(ops.Iterator()), ", "),
		)
	}

	return out, nil
}

func RunCommands(ctx context.Context, cmds stw.Slice[Command]) error {
	size := cmds.Len()
	switch {
	case size == 1:
		return ers.Wrapf(stw.Ptr(cmds[0]).Worker().Run(ctx), "running command %q", cmds[0])
	case size < runtime.NumCPU():
		return ers.Wrapf(TOOLS.CommandPool(irt.Slice(cmds), wpa.WorkerGroupConfNumWorkers(size)).Run(ctx), "running %d batch of commands %s", size, cmds)
	default:
		return ers.Wrapf(TOOLS.CommandPool(irt.Slice(cmds)).Run(ctx), "running %d batch of commands %s", size, cmds)
	}
}
