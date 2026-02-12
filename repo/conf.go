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
	GitRepos  stw.Slice[GitRepository]           `bson:"git" json:"git" yaml:"git"`
	Projects  []Project                          `bson:"projects" json:"projects" yaml:"projects"`
	TagGroups stw.Map[string, stw.Slice[string]] `bson:"tag_groups" json:"tag_groups" yaml:"tag_groups"`

	lookupProcessed bool
	caches          struct {
		tags       stw.Map[string, stw.Slice[string]] // tag names to repo names
		lookup     stw.Map[string, GitRepository]     // repo names to repositories
		validation adt.Once[error]
	}
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Do(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	ec.Push(conf.projectsValidate())

	for idx := range conf.GitRepos {
		ec.Wrapf((&conf.GitRepos[idx]).Validate(), "%d/%d of %T is not valid", idx, len(conf.GitRepos), conf.GitRepos[idx])
	}

	ec.Push(conf.rebuildIndexes())

	for group, tags := range conf.TagGroups {
		ec.Whenf(conf.caches.lookup.Check(group), "group name %q is an existing repo name", group)
		ec.Whenf(conf.caches.tags.Check(group), "group name %q is an existing tag name", group)
		for _, tg := range tags {
			ec.Whenf(!conf.caches.lookup.Check(tg), "tag %q in group %q is NOT an existing repo name", tg, group)
			ec.Whenf(!conf.caches.tags.Check(tg), "tag name %q is NOT an existing tag name", group)
		}
	}

	return ec.Resolve()
}

func (conf *Configuration) Join(mcf *Configuration) {
	conf.GitRepos.Extend(irt.Slice(mcf.GitRepos))
	conf.TagGroups.Extend(mcf.TagGroups.Iterator())
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

func (conf *Configuration) rebuildIndexes() error {
	if conf.lookupProcessed {
		return nil
	}
	defer func() { conf.lookupProcessed = true }()
	var ec erc.Collector

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

	for gt, tags := range conf.TagGroups.Iterator() {
		group := dt.Set[string]{}
		for tag := range irt.Slice(tags) {
			group.Extend(irt.Slice(conf.caches.tags.Get(tag)))
			if conf.caches.lookup.Check(tag) {
				group.Add(tag)
			}
		}
		conf.caches.tags.Store(gt, irt.Collect(group.Iterator(), group.Len()))
		ec.Whenf(group.Len() == 0, "for tag group %q, no resolveable tags", gt)
	}
	return ec.Resolve()
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
