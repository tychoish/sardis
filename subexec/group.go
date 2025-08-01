package subexec

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/grip"
	jutil "github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/util"
)

type Group struct {
	Category       string                 `bson:"category" json:"category" yaml:"category"`
	Name           string                 `bson:"name" json:"name" yaml:"name"`
	Aliases        []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory      string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment    dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	CmdNamePrefix  string                 `bson:"command_name_prefix" json:"command_name_prefix" yaml:"command_name_prefix"`
	Command        string                 `bson:"default_command" json:"default_command" yaml:"default_command"`
	Notify         *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background     *bool                  `bson:"background" json:"background" yaml:"background"`
	Host           *string                `bson:"host" json:"host" yaml:"host"`
	Commands       dt.Slice[Command]      `bson:"commands" json:"commands" yaml:"commands"`
	MenuSelections []string               `bson:"menu" json:"menu" yaml:"menu"`
	SortHint       int                    `bson:"sort_hint" json:"sort_hint" yaml:"sort_hint"`
	Synthetic      bool                   `bson:"-" json:"-" yaml:"-"`
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
	fun.Invariant.Ok(idx >= 0 && idx < len(cg.Commands), "command out of bounds", cg.Name)
	ops := []string{}

	for _, grp := range append([]string{cg.Name}, cg.Aliases...) {
		ops = append(ops, util.DotJoin(cg.Category, grp, cg.Commands[idx].Name))
	}

	return ops
}

func (cg *Group) Selectors() []string {
	set := &dt.Set[string]{}
	set.Order()

	for cmd := range slices.Values(cg.Commands) {
		set.Add(cmd.Name)
	}

	return fun.NewGenerator(set.Stream().Slice).Force().Resolve()
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

	ec.When(cg.Name == "", ers.Error("command group must have name"))

	for idx := range cg.Commands {
		cmd := cg.Commands[idx]
		cmd.GroupCategory = cg.Category
		cmd.GroupName = cg.Name
		cmd.Notify = ft.Default(cmd.Notify, cg.Notify)
		cmd.Background = ft.Default(cmd.Background, cg.Background)
		cmd.Directory = jutil.TryExpandHomedir(ft.Default(cmd.Directory, home))

		ec.Whenf(cmd.Name == "", "command in group [%s](%d) must have a name", cg.Name, idx)
		ec.Whenf(cmd.Command == "" && cmd.OverrideDefault, "cannot override default without an override, in group [%s] command [%s] at index (%d)", cg.Name, cmd.Name, idx)

		if cg.Environment != nil || cmd.Environment != nil {
			env := dt.Map[string, string]{}
			if cg.Environment != nil {
				env.ExtendWithStream(cg.Environment.Stream()).Ignore().Wait()
			}

			if cmd.Environment != nil {
				env.ExtendWithStream(cmd.Environment.Stream()).Ignore().Wait()
			}

			cmd.Environment = env
		}

		if ft.Not(cmd.OverrideDefault) {
			cmd.Command = ft.Default(cmd.Command, cg.Command)
			cmd.Command = ft.Default(cmd.Command, cmd.Name)
			cmd.Command = ft.Default(strings.ReplaceAll(cg.Command, "{{command}}", cmd.Command), cmd.Command)
		}

		if cc := cmd.Command; strings.Contains(cc, "{{") && strings.Contains(cc, "}}") {
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{name}}", cmd.Name)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.category}}", cg.Category)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.name}}", cg.Name)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{host}}", ft.Ref(cg.Host))
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{prefix}}", cg.CmdNamePrefix)
		}

		for idx := range cmd.Commands {
			if cc := cmd.Commands[idx]; strings.Contains(cc, "{{") && strings.Contains(cc, "}}") {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{command}}", cmd.Command)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{name}}", cmd.Name)
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{host}}", ft.Ref(cg.Host))
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
	cg.SortHint = intish.AbsMax(cg.SortHint, rhv.SortHint)

	return true
}

func FilterCommands(cmds dt.Slice[Command], args []string) (dt.Slice[Command], error) {
	ops := dt.NewSetFromSlice(args)
	switch {
	case len(cmds) == 0:
		return nil, ers.New("cannot resolve commands without input commands")
	case len(args) == 0:
		return nil, ers.New("must specify one or more commands to resolve")
	case ops.Len() != len(args):
		return nil, fmt.Errorf("ambiguous input with %d duplicate items %s", ops.Len()-len(args), args)
	}

	seen := dt.Set[string]{}
	seen.Order()

	out := cmds.Filter(func(cmd Command) bool {
		name := cmd.NamePrime()
		return ops.Check(name) && !seen.AddCheck(name)
	})

	// if we didn't find all that we were looking for?
	if ops.Len() != len(out) {
		return nil, fmt.Errorf("found %d [%s] ops, of %d [%s] arguments",
			len(out), strings.Join(fun.NewGenerator(seen.Stream().Slice).Force().Resolve(), ", "),
			ops.Len(), strings.Join(fun.NewGenerator(ops.Stream().Slice).Force().Resolve(), ", "),
		)
	}

	return out, nil
}

func RunCommands(ctx context.Context, cmds dt.Slice[Command]) error {
	size := cmds.Len()
	switch {
	case size == 1:
		return ers.Wrapf(ft.Ptr(cmds[0]).Worker().Run(ctx), "running command %q", cmds[0])
	case size < runtime.NumCPU():
		return ers.Wrapf(
			fun.MAKE.WorkerPool(TOOLS.Converter().Stream(cmds.Stream())).Run(ctx),
			"running small %d batch  of commands %s", size, cmds,
		)
	default:
		return ers.Wrapf(TOOLS.CommandPool(cmds.Stream()).Run(ctx), "running  %d batch of commands %s", size, cmds)
	}
}
