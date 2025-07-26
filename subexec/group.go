package subexec

import (
	"slices"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/util"
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

func dotJoin(elems ...string) string       { return dotJoinParts(elems) }
func dotJoinParts(elems []string) string   { return strings.Join(removeZeros(elems), ".") }
func dotSplit(in string) []string          { return strings.Split(in, ".") }
func dotSplitN(in string, n int) []string  { return strings.SplitN(in, ".", n) } // nolint:unused
func removeZeros[T comparable](in []T) []T { return slices.DeleteFunc(in, isZero) }
func isZero[T comparable](i T) bool        { var z T; return i == z }

func (cg *Group) ResolvedCategory() string {
	if cg.Category != "" {
		return cg.Category
	}
	return cg.Name
}
func (cg *Group) ID() string       { return dotJoinParts(cg.IDPath()) }
func (cg *Group) IDPath() []string { return []string{cg.Category, cg.Name, cg.CmdNamePrefix} }

func (cg *Group) NamesAtIndex(idx int) []string {
	fun.Invariant.Ok(idx >= 0 && idx < len(cg.Commands), "command out of bounds", cg.Name)
	ops := []string{}

	for _, grp := range append([]string{cg.Name}, cg.Aliases...) {
		ops = append(ops, dotJoin(cg.Category, grp, cg.Commands[idx].Name))
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
	home := util.GetHomedir()
	ec := &erc.Collector{}

	for _, selection := range cg.MenuSelections {
		cg.Commands = append(cg.Commands, Command{Name: selection, Command: selection})
	}

	if cg.Category == "" && cg.CmdNamePrefix != "" {
		cg.Category = cg.Name
		cg.Name = cg.CmdNamePrefix
		cg.CmdNamePrefix = ""
	}

	if cg.Name == "" && cg.CmdNamePrefix != "" {
		grip.Warningf("command group cat=%q prefix=%q is missing name, rotating name", cg.Category, cg.CmdNamePrefix)
		cg.Name = cg.CmdNamePrefix
		cg.CmdNamePrefix = ""
	}

	ec.When(cg.Name == "", ers.Error("command group must have name"))

	for idx := range cg.Commands {
		cmd := cg.Commands[idx]
		cmd.GroupName = cg.Name
		cmd.Notify = ft.Default(cmd.Notify, cg.Notify)
		cmd.Background = ft.Default(cmd.Background, cg.Background)
		cmd.Directory = util.TryExpandHomedir(ft.Default(cmd.Directory, home))

		ec.Whenf(cmd.Name == "", "command in group [%s](%d) must have a name", cg.Name, idx)
		ec.Whenf(cmd.Command == "" && cmd.OverrideDefault, "cannot override default without an override, in group [%s] command [%s] at index (%d)", cg.Name, cmd.Name, idx)

		// ec.Whenf(cmd.Command != "" && cmd.WorkerDefinition != nil, "invalid definition in group [%s] for [%s] at index (%d) [%q]", cg.Name, cmd.Name, idx, cmd.Command)
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

		cmd.Name = dotJoin(cg.CmdNamePrefix, cmd.Name)

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
