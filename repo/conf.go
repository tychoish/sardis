package repo

import (
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
)

type Configuration struct {
	GitRepos dt.Slice[GitRepository] `bson:"git" json:"git" yaml:"git"`

	lookupProcessed bool
	caches          struct {
		tags       dt.Map[string, dt.Slice[*GitRepository]]
		lookup     dt.Map[string, dt.Slice[*GitRepository]]
		validation adt.Once[error]
	}
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	for idx := range conf.GitRepos {
		repo := &conf.GitRepos[idx]
		ec.Wrapf(repo.Validate(), "%d/%d of %T is not valid", idx, len(conf.GitRepos), conf.GitRepos[idx])
	}

	conf.rebuildIndexes()
	return ec.Resolve()
}

func (conf *Configuration) FindOne(name string) (*GitRepository, error) {
	repos, ok := conf.LookupTable().Load(name)
	if !ok {
		return nil, fmt.Errorf("repo %q is not defined", name)
	}

	if l := repos.Len(); l != 1 {
		return nil, fmt.Errorf("ambiguity: found %d repos matching %q", l, name)
	}
	return repos.Index(0), nil
}

func (conf *Configuration) Tags() dt.Map[string, dt.Slice[*GitRepository]] {
	conf.rebuildIndexes()
	return conf.caches.tags
}

func (conf *Configuration) LookupTable() dt.Map[string, dt.Slice[*GitRepository]] {
	conf.rebuildIndexes()
	return conf.caches.lookup
}

func (conf *Configuration) rebuildIndexes() {
	if conf.lookupProcessed {
		return
	}
	defer func() { conf.lookupProcessed = true }()

	conf.caches.tags = make(map[string]dt.Slice[*GitRepository])
	conf.caches.lookup = make(map[string]dt.Slice[*GitRepository])
	for idx := range conf.GitRepos {
		rp := conf.GitRepos[idx]
		for _, tag := range conf.GitRepos[idx].Tags {
			conf.caches.tags[tag] = append(conf.caches.tags[tag], ft.Ptr(rp))
			conf.caches.lookup[tag] = append(conf.caches.lookup[tag], ft.Ptr(rp))
		}

		name := conf.GitRepos[idx].Name
		conf.caches.lookup[name] = append(conf.caches.lookup[name], ft.Ptr(rp))
	}
}

func (conf *Configuration) FindAll(ids ...string) dt.Slice[GitRepository] {
	if len(ids) == 0 {
		return nil
	}

	matching := dt.Map[string, GitRepository]{}

	lookup := conf.LookupTable()
	if lookup.Len() == 0 {
		return nil
	}

	for _, id := range ids {
		tagged, ok := lookup.Load(id)
		if !ok || tagged.Len() == 0 {
			continue
		}

		tagged.ReadAll(func(pt *GitRepository) {
			if pt != nil {
				matching[pt.Name] = ft.Ref(pt)
			}
		})
	}

	return fun.NewGenerator(matching.Values().Slice).Force().Resolve()
}
