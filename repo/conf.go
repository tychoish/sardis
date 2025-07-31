package repo

import (
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
)

type Configuration struct {
	GitRepos dt.Slice[GitRepository] `bson:"git" json:"git" yaml:"git"`
	Projects []Project               `bson:"projects" json:"projects" yaml:"projects"`

	lookupProcessed bool
	caches          struct {
		tags       dt.Map[string, dt.Slice[string]] // tag names to repo names
		lookup     dt.Map[string, GitRepository]    // repo names to repositories
		validation adt.Once[error]
	}
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	ec.Push(conf.projectsValidate())
	for idx := range conf.GitRepos {
		repo := &conf.GitRepos[idx]
		ec.Wrapf(repo.Validate(), "%d/%d of %T is not valid", idx, len(conf.GitRepos), conf.GitRepos[idx])
	}

	conf.rebuildIndexes()
	return ec.Resolve()
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

func (conf *Configuration) Tags() dt.Slice[string] {
	conf.rebuildIndexes()

	return fun.NewGenerator(conf.caches.tags.Keys().Slice).Capture().Resolve()
}

func (conf *Configuration) rebuildIndexes() {
	if conf.lookupProcessed {
		return
	}
	defer func() { conf.lookupProcessed = true }()

	conf.caches.tags = make(map[string]dt.Slice[string])
	conf.caches.lookup = make(map[string]GitRepository)
	for idx := range conf.GitRepos {
		rp := conf.GitRepos[idx]

		fun.Invariant.IsFalse(conf.caches.lookup.Check(rp.Name), "duplicate repositoriy named", rp.Name)

		conf.caches.lookup.Add(rp.Name, rp)

		for _, tg := range rp.Tags {
			rps := conf.caches.tags[tg]
			rps.Add(rp.Name)
			conf.caches.tags[tg] = rps
		}
	}
}

func (conf *Configuration) FindAll(ids ...string) dt.Slice[GitRepository] {
	if len(ids) == 0 {
		return nil
	}

	matching := dt.Slice[GitRepository]{}
	seen := dt.Set[string]{}

	for _, id := range ids {
		if rp, ok := conf.caches.lookup[id]; ok {
			if !seen.AddCheck(rp.Name) {
				matching.Add(rp)
			}
			continue
		}

		for _, rtn := range conf.caches.tags[id] {
			if !seen.AddCheck(rtn) && conf.caches.lookup.Check(rtn) {
				matching.Add(conf.caches.lookup.Get(rtn))
			}
		}
	}

	return matching
}
