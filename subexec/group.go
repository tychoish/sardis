package subexec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/jasper/util"
)

type Group struct {
	Category      string                 `bson:"category" json:"category" yaml:"category"`
	Name          string                 `bson:"name" json:"name" yaml:"name"`
	Aliases       []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory     string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment   dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	CmdNamePrefix string                 `bson:"command_name_prefix" json:"command_name_prefix" yaml:"command_name_prefix"`
	Command       string                 `bson:"default_command" json:"default_command" yaml:"default_command"`
	Notify        *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background    *bool                  `bson:"background" json:"background" yaml:"background"`
	Host          *string                `bson:"host" json:"host" yaml:"host"`
	Commands      []Command              `bson:"commands" json:"commands" yaml:"commands"`

	unaliasedName string
	// TODO populate the export cache.
	exportCache *adt.Once[map[string]Command]
}

func dotJoin(elems ...string) string      { return dotJoinParts(elems) }
func dotJoinParts(elems []string) string  { return strings.Join(elems, ".") }
func dotSpit(in string) []string          { return strings.Split(in, ".") }
func dotSplitN(in string, n int) []string { return strings.SplitN(in, ".", n) }

func (cg *Group) ID() string       { return dotJoinParts(cg.IDPath()) }
func (cg *Group) IDPath() []string { return []string{cg.Category, cg.Name, cg.CmdNamePrefix} }

func (cg *Group) NamesAtIndex(idx int) []string {
	fun.Invariant.Ok(idx >= 0 && idx < len(cg.Commands), "command out of bounds", cg.Name)
	ops := []string{}

	for _, grp := range append([]string{cg.Name}, cg.Aliases...) {
		cmd := cg.Commands[idx]
		for _, name := range append([]string{cmd.Name}, cmd.Aliases...) {
			ops = append(ops, dotJoin(cg.Category, grp, name))
		}
	}

	return ops
}

func (cg *Group) Selectors() []string {
	set := &dt.Set[string]{}
	for cmd := range slices.Values(cg.Commands) {
		set.Add(cmd.Name)
	}

	return fun.NewGenerator(set.Stream().Slice).Force().Resolve()
}

func (cg *Group) Validate() error {
	var err error
	home := util.GetHomedir()
	ec := &erc.Collector{}

	ec.When(cg.Name == "", ers.Error("command group must have name"))
	if cg.unaliasedName == "" {
		cg.unaliasedName = cg.Name
	}

	var aliased []Command

	for idx := range cg.Commands {
		cmd := cg.Commands[idx]
		cmd.GroupName = cg.Name

		ec.Whenf(cmd.Name == "", "command in group [%s](%d) must have a name", cg.Name, idx)

		if cmd.Directory == "" {
			cmd.Directory = home
		}

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

		if cmd.Command == "" {
			// THIS LOGIC IS WEIRD: there are lots of
			//   cases where you want (potentially) the
			//   exact opposite, that you want the command
			//   to be the name __if__ there is a default
			//   command specified. (For templating and
			//   replacecment reasons).
			//
			//   TODO investigating
			//      inverting this.
			ec.Whenf(cmd.OverrideDefault, "cannot override default without an override, in group [%s] command [%s](%d)", cg.Name, cmd.Name, idx)
			if cg.Command != "" {
				cmd.Command = cg.Command
			} else {
				cmd.Command = cmd.Name
			}
			ec.Whenf(cmd.Command == "", "cannot resolve default command in group [%s] command [%s](%d)", cg.Name, cmd.Name, idx)
		}

		ec.Whenf(strings.Contains(cmd.Command, " {{command}}"), "unresolveable token found in group [%s] command [%s](%d)", cg.Name, cmd.Name, idx)

		if cg.Command != "" && !cmd.OverrideDefault {
			cmd.Command = strings.ReplaceAll(cg.Command, "{{command}}", cmd.Command)
		}

		cmd.Command = strings.ReplaceAll(cmd.Command, "{{name}}", cmd.Name)
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.name}}", cg.Name)
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{host}}", ft.Ref(cg.Host))
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{prefix}}", cg.CmdNamePrefix)

		if len(cmd.Aliases) >= 1 {
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{alias}}", cmd.Aliases[0])
		}
		for idx, alias := range cmd.Aliases {
			cmd.Command = strings.ReplaceAll(cmd.Command, fmt.Sprintf("{{alias[%d]}}", idx), alias)
		}

		for idx := range cmd.Commands {
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{command}}", cmd.Command)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{name}}", cmd.Name)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{host}}", ft.Ref(cg.Host))
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{group.name}}", cg.Name)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{prefix}}", cg.Name)

			if len(cmd.Aliases) >= 1 {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{alias}}", cmd.Aliases[0])
			}

			for idx, alias := range cmd.Aliases {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], fmt.Sprintf("{{alias[%d]}}", idx), alias)
			}
		}

		cmd.Notify = ft.Default(cmd.Notify, cg.Notify)
		cmd.Background = ft.Default(cmd.Background, cg.Background)
		cmd.Directory, err = homedir.Expand(cmd.Directory)
		ec.Add(ers.Wrapf(err, "command group(%s)  %q at %d", cmd.GroupName, cmd.Name, idx))

		for _, alias := range cmd.Aliases {
			acmd := cmd
			if cg.CmdNamePrefix != "" {
				acmd.Name = fmt.Sprintf("%s.%s", cg.CmdNamePrefix, alias)
			} else {
				acmd.Name = alias
			}
			acmd.Aliases = nil
			acmd.unaliasedName = cmd.Name
			aliased = append(aliased, acmd)
		}

		if cg.CmdNamePrefix != "" {
			cmd.Name = fmt.Sprintf("%s.%s", cg.CmdNamePrefix, cmd.Name)
		}
		cmd.Aliases = nil
		cg.Commands[idx] = cmd
	}
	cg.Commands = append(cg.Commands, aliased...)

	return ec.Resolve()
}

func (cg *Group) doMerge(rhv Group) bool {
	if cg.Name != rhv.Name {
		return false
	}

	cg.Commands = append(cg.Commands, rhv.Commands...)
	cg.Command = ""
	cg.Aliases = nil
	cg.Environment = nil
	return true
}
