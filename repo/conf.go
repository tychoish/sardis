package repo

import (
	"fmt"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

type Configuration struct {
	GitRepos stw.Slice[GitRepository] `bson:"git" json:"git" yaml:"git"`
	Projects []Project                `bson:"projects" json:"projects" yaml:"projects"`

	lookupProcessed bool
	caches          struct {
		tags       stw.Map[string, stw.Slice[string]] // tag names to repo names
		lookup     stw.Map[string, GitRepository]     // repo names to repositories
		validation adt.Once[error]
	}
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	ec.Push(conf.projectsValidate())

	for idx := range conf.GitRepos {
		ec.Wrapf((&conf.GitRepos[idx]).Validate(), "%d/%d of %T is not valid", idx, len(conf.GitRepos), conf.GitRepos[idx])
	}

	conf.rebuildIndexes()
	return ec.Resolve()
}

func (conf *Configuration) Join(mcf *Configuration) {
	conf.GitRepos = append(conf.GitRepos, mcf.GitRepos...)
	conf.Projects = append(conf.Projects, mcf.Projects...)
}

func (conf *Configuration) FindOne(name string) (*GitRepository, error) {
	if rp, ok := conf.caches.lookup.Load(name); ok {
		return &rp, nil
	}

	if tagged, ok := conf.caches.tags.Load(name); ok {
		if l := tagged.Len(); l == 1 {
			return conf.FindOne(tagged.Index(0))
		} else if l > 1 {
			return nil, fmt.Errorf("found %d repos tagged %q", l, name)
		}
	}

	return nil, fmt.Errorf("no repository named %q", name)
}

func (conf *Configuration) Tags() stw.Slice[string] {
	conf.rebuildIndexes()

	return irt.Collect(conf.caches.tags.Keys())
}

func (conf *Configuration) rebuildIndexes() {
	if conf.lookupProcessed {
		return
	}
	defer func() { conf.lookupProcessed = true }()

	conf.caches.tags = make(map[string]stw.Slice[string])
	conf.caches.lookup = make(map[string]GitRepository)
	for idx := range conf.GitRepos {
		rp := conf.GitRepos[idx]

		erc.InvariantOk(!conf.caches.lookup.Check(rp.Name), "duplicate repositoriy named", rp.Name)

		conf.caches.lookup.Store(rp.Name, rp)

		for _, tg := range rp.Tags {
			rps := conf.caches.tags[tg]
			rps.Push(rp.Name)
			conf.caches.tags[tg] = rps
		}
	}
}

func (conf *Configuration) FindAll(ids ...string) stw.Slice[GitRepository] {
	if len(ids) == 0 {
		return nil
	}

	matching := stw.Slice[GitRepository]{}
	seen := dt.Set[string]{}

	for _, id := range ids {
		if rp, ok := conf.caches.lookup[id]; ok {
			if !seen.Add(rp.Name) {
				matching.Push(rp)
			}
			continue
		}

		for _, rtn := range conf.caches.tags[id] {
			if !seen.Add(rtn) && conf.caches.lookup.Check(rtn) {
				matching.Push(conf.caches.lookup.Get(rtn))
			}
		}
	}

	return matching
}
