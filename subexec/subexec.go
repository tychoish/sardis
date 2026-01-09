package subexec

import (
	stdcmp "cmp"
	"slices"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Commands stw.Slice[Group] `bson:"groups" json:"groups" yaml:"groups"`

	Settings struct {
		SSHAgentSocketPath    string `bson:"ssh_agent_socket_path" json:"ssh_agent_socket_path" yaml:"ssh_agent_socket_path"`
		AlacrittySocketPath   string `bson:"alacritty_socket_path" json:"alacritty_socket_path" yaml:"alacritty_socket_path"`
		IncludeLocalSHH       *bool  `bson:"include_local_ssh" json:"include_local_ssh" yaml:"include_local_ssh"`
		AllowUndefinedSockets *bool  `bson:"allow_undefined_sockets" json:"allow_undefined_sockets" yaml:"allow_undefined_sockets"`
	} `bson:"settings" json:"settings" yaml:"settings"`

	caches struct {
		commandGroups       adt.Once[map[string]Group]
		allCommdands        adt.Once[stw.Slice[Command]]
		comandGroupNames    adt.Once[[]string]
		validation          adt.Once[error]
		sshAgentPath        adt.Once[string]
		alacrittySocketPath adt.Once[string]
	}
}

func (conf *Configuration) AlacrittySocket() string { return conf.caches.alacrittySocketPath.Resolve() }
func (conf *Configuration) SSHAgentSocket() string  { return conf.caches.sshAgentPath.Resolve() }

func (conf *Configuration) Join(mcf *Configuration) {
	conf.Settings.AlacrittySocketPath = util.Default(mcf.Settings.AlacrittySocketPath, conf.Settings.AlacrittySocketPath)
	conf.Settings.SSHAgentSocketPath = util.Default(mcf.Settings.SSHAgentSocketPath, conf.Settings.SSHAgentSocketPath)
	conf.Settings.IncludeLocalSHH = util.Default(mcf.Settings.IncludeLocalSHH, conf.Settings.IncludeLocalSHH)
	conf.Settings.AllowUndefinedSockets = util.Default(mcf.Settings.AllowUndefinedSockets, conf.Settings.AllowUndefinedSockets)

	conf.Commands = append(conf.Commands, mcf.Commands...)
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}

	for idx := range conf.Commands {
		ec.Wrapf(conf.Commands[idx].Validate(), "%d of %T is not valid", idx, conf.Commands[idx])
	}
	ec.Push(conf.resolveAliasesAndMergeGroups())

	conf.caches.alacrittySocketPath.Set(func() string {
		if conf.Settings.AlacrittySocketPath != "" {
			return conf.Settings.AlacrittySocketPath
		}
		path, err := util.GetAlacrittySocketPath()
		switch {
		case err == nil:
			return path
		case stw.DerefZ(conf.Settings.AllowUndefinedSockets):
			return ""
		default:
			erc.Invariant(err)
		}
		// unreachable
		return ""
	})

	conf.caches.sshAgentPath.Set(func() string {
		if conf.Settings.SSHAgentSocketPath != "" {
			return conf.Settings.SSHAgentSocketPath
		}
		path, err := util.GetSSHAgentPath()
		switch {
		case err == nil:
			return path
		case stw.DerefZ(conf.Settings.AllowUndefinedSockets):
			return ""
		default:
			erc.Invariant(err)
		}
		// unreachable
		return ""
	})

	return ec.Resolve()
}

func (conf *Configuration) resolveAliasesAndMergeGroups() error {
	// expand aliases
	if len(conf.Commands) == 0 {
		return nil
	}
	hostname := util.GetHostname()
	withAliases := make([]Group, 0, len(conf.Commands)+len(conf.Commands)/2+1)
	for idx := range conf.Commands {
		cg := conf.Commands[idx]
		if cg.Host != nil && !stw.DerefZ(conf.Settings.IncludeLocalSHH) {
			chost := stw.DerefZ(cg.Host)
			if chost != "" && chost == hostname {
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
		lhn := util.DotJoin(conf.Commands[idx].Category, conf.Commands[idx].Name)

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
	sparse := irt.Collect(irt.KVjoin(irt.Map(index)))

	// reorder it because it came off of a default map in random order
	slices.SortStableFunc(sparse, irt.KVcmpSecond)

	resolved := irt.Collect(
		irt.ForEach(
			// resolve the merged groups
			irt.Convert(
				// the value in the sparse map are the indexes of the merged groups
				irt.Second(irt.KVsplit(irt.Slice(sparse))),
				// use the .Index accessor to pull the groups out of the
				// stream of sparse indexes of now merged groups ⬇️
				conf.Commands.Index,
			),
			func(grp Group) {
				slices.SortStableFunc(
					grp.Commands,
					func(lhv, rhv Command) (cc int) {
						cc = stdcmp.Compare(rhv.SortHint, lhv.SortHint)
						if cc == 0 {
							return stdcmp.Compare(lhv.Name, rhv.Name)
						}
						return cc
					},
				)
			},
		),
	)

	slices.SortStableFunc(resolved, func(lhv, rhv Group) int {
		switch {
		case lhv.Host != rhv.Host && (lhv.Host != nil || rhv.Host != nil):
			return stdcmp.Compare(stw.DerefZ(lhv.Host), stw.DerefZ(rhv.Host))
		case lhv.Synthetic != rhv.Synthetic:
			if lhv.Synthetic {
				return 1
			}
			return -1
		case lhv.SortHint != rhv.SortHint:
			return stdcmp.Compare(rhv.SortHint, lhv.SortHint)
		case lhv.Category != rhv.Category:
			return stdcmp.Compare(lhv.Category, rhv.Category)
		case lhv.Name != rhv.Name:
			return stdcmp.Compare(lhv.Name, rhv.Name)
		case lhv.CmdNamePrefix != rhv.CmdNamePrefix:
			return stdcmp.Compare(lhv.CmdNamePrefix, rhv.CmdNamePrefix)
		case len(lhv.Commands) != len(rhv.Commands):
			return stdcmp.Compare(len(lhv.Commands), len(rhv.Commands))
		default:
			return 0
		}
	})

	conf.Commands = resolved
	return nil
}

func (conf *Configuration) ExportAllCommands() stw.Slice[Command] {
	return conf.caches.allCommdands.Call(conf.doExportAllCommands)
}

func (conf *Configuration) doExportAllCommands() stw.Slice[Command] {
	out := make([]Command, 0, len(conf.Commands)*4)
	host := util.GetHostname()

	for _, grp := range conf.Commands {
		hn, ok := stw.DerefOk(grp.Host)
		if ok && hn != "" && hn == host && !stw.DerefZ(conf.Settings.IncludeLocalSHH) {
			continue
		}

		for cidx := range grp.Commands {
			cmd := grp.Commands[cidx]
			cmd.Name = util.DotJoin(grp.Category, grp.Name, cmd.Name)
			out = append(out, cmd)
		}
	}

	return out
}

func (conf *Configuration) ExportCommandGroups() stw.Map[string, Group] {
	return conf.caches.commandGroups.Call(conf.doExportCommandGroups)
}

func (conf *Configuration) ExportGroupNames() stw.Slice[string] {
	return conf.caches.comandGroupNames.Call(conf.doExportGroupNames)
}

func (conf *Configuration) doExportGroupNames() []string {
	return irt.Collect(conf.ExportCommandGroups().Keys())
}

func (conf *Configuration) doExportCommandGroups() map[string]Group {
	out := make(map[string]Group, len(conf.Commands))
	hostname := util.GetHostname()
	for idx := range conf.Commands {
		group := conf.Commands[idx]
		hn, ok := stw.DerefOk(group.Host)
		if ok && hn != "" && hn == hostname && !stw.DerefZ(conf.Settings.IncludeLocalSHH) {
			continue
		}

		out[util.DotJoin(group.Category, group.Name)] = group

		for _, alias := range group.Aliases {
			ag := conf.Commands[idx]
			out[alias] = ag
		}
	}
	return out
}
