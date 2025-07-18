package subexec

import (
	"fmt"
	"os"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Commands []Group `bson:"groups" json:"groups" yaml:"groups"`
	Settings struct {
		SSHAgentSocketPath  string `bson:"ssh_agent_socket_path" json:"ssh_agent_socket_path" yaml:"ssh_agent_socket_path"`
		AlacrittySocketPath string `bson:"alacritty_socket_path" json:"alacritty_socket_path" yaml:"alacritty_socket_path"`
		IncludeLocalSHH     bool   `bson:"include_local_ssh" json:"include_local_ssh" yaml:"include_local_ssh"`
		Hostname            string `bson:"-" json:"-" yaml:"-"`
	}

	caches struct {
		commandGroups       adt.Once[map[string]Group]
		allCommdands        adt.Once[[]Command]
		comandGroupNames    adt.Once[[]string]
		validation          adt.Once[error]
		sshAgentPath        adt.Once[string]
		alacrittySocketPath adt.Once[string]
	}
}

func (conf *Configuration) AlacrittySocket() string { return conf.caches.alacrittySocketPath.Resolve() }
func (conf *Configuration) SSHAgentSocket() string  { return conf.caches.sshAgentPath.Resolve() }

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}

	for idx := range conf.Commands {
		ec.Wrapf(conf.Commands[idx].Validate(), "%d of %T is not valid", idx, conf.Commands[idx])
	}

	ec.Push(conf.resolveAliasesAndMergeGroups())

	conf.Settings.Hostname = makeErrorHandler[string](ec.Push)(os.Hostname())

	conf.caches.alacrittySocketPath.Set(func() string {
		if conf.Settings.AlacrittySocketPath != "" {
			return conf.Settings.AlacrittySocketPath
		}
		return ft.Must(util.GetAlacrittySocketPath())
	})

	conf.caches.sshAgentPath.Set(func() string {
		if conf.Settings.SSHAgentSocketPath != "" {
			return conf.Settings.SSHAgentSocketPath
		}
		return ft.Must(util.GetSSHAgentPath())
	})

	return ec.Resolve()
}

func makeErrorHandler[T any](eh func(error)) func(T, error) T {
	return func(v T, err error) T { eh(err); return v }
}

func (conf *Configuration) resolveAliasesAndMergeGroups() error {
	// expand aliases
	if len(conf.Commands) == 0 {
		return nil
	}
	withAliases := make([]Group, 0, len(conf.Commands)+len(conf.Commands)/2+1)
	for idx := range conf.Commands {
		cg := conf.Commands[idx]
		if cg.Host != nil && !conf.Settings.IncludeLocalSHH {
			chost := ft.Ref(cg.Host)
			if chost != "" && chost == conf.Settings.Hostname {
				continue
			}
		}

		withAliases = append(withAliases, cg)

		for _, alias := range cg.Aliases {
			acg := cg
			acg.Aliases = nil
			acg.Name = alias
			withAliases = append(withAliases, acg)
		}
		cg.Aliases = nil
	}
	conf.Commands = withAliases

	index := make(map[string]int, len(conf.Commands))
	haveMerged := false
	for idx := range conf.Commands {
		lhn := conf.Commands[idx].Name

		if _, ok := index[lhn]; !ok {
			index[lhn] = idx
			continue
		}

		cg := &conf.Commands[index[lhn]]
		cg.doMerge(conf.Commands[idx])
		conf.Commands[index[lhn]] = *cg
		haveMerged = true
	}

	if !haveMerged {
		return nil
	}

	// get map of names -> indexes as an ordered sequence
	sparse := dt.NewMap(index).Pairs()

	// reorder it because it came off of a default map in random order
	sparse.SortQuick(cmp.LessThanConverter(func(p dt.Pair[string, int]) int { return p.Value }))

	// make an output structure
	resolved := dt.NewSlice(make([]Group, 0, len(index)))

	// read the resolution inside out...
	//
	// ⬇️ ingest the contents of the converted and reordered stream
	// into the resolved slice
	err := resolved.Populate(
		// use the .Index accessor to pull the groups out of the
		// stream of sparse indexes of now merged groups ⬇️
		fun.MakeConverter(dt.NewSlice(conf.Commands).Index).Stream(
			// ⬇️ convert it into the (sparse) indexes of merged groups ⬆
			fun.MakeConverter(func(p dt.Pair[string, int]) int { return p.Value }).Stream(
				// ⬇️ take the (now ordered) pairs that we merged and ⬆
				sparse.Stream(),
			),
		),
	).Wait()

	if err != nil {
		return err
	}

	conf.Commands = resolved
	return nil
}

func (conf *Configuration) ExportAllCommands() []Command {
	return conf.caches.allCommdands.Call(conf.doExportAllCommands)
}
func (conf *Configuration) doExportAllCommands() []Command {
	out := dt.NewSlice([]Command{})

	for _, grp := range conf.Commands {
		hn, ok := ft.RefOk(grp.Host)
		if ok && hn != "" && hn == conf.Settings.Hostname && !conf.Settings.IncludeLocalSHH {
			continue
		}

		for cidx := range grp.Commands {
			cmd := grp.Commands[cidx]
			cmd.Name = fmt.Sprintf("%s.%s", grp.Name, cmd.Name)
			out = append(out, cmd)
		}
	}

	return out
}

func (conf *Configuration) ExportCommandGroups() dt.Map[string, Group] {
	return conf.caches.commandGroups.Call(conf.doExportCommandGroups)
}

func (conf *Configuration) ExportGroupNames() dt.Slice[string] {
	return conf.caches.comandGroupNames.Call(conf.doExportGroupNames)
}

func (conf *Configuration) doExportGroupNames() []string {
	return fun.NewGenerator(conf.ExportCommandGroups().Keys().Slice).Force().Resolve()
}

func (conf *Configuration) doExportCommandGroups() map[string]Group {
	out := make(map[string]Group, len(conf.Commands))
	for idx := range conf.Commands {
		group := conf.Commands[idx]
		hn, ok := ft.RefOk(group.Host)
		if ok && hn != "" && hn == conf.Settings.Hostname && !conf.Settings.IncludeLocalSHH {
			continue
		}

		out[group.Name] = group
		for _, alias := range group.Aliases {
			ag := conf.Commands[idx]
			out[alias] = ag
		}
	}
	return out
}
