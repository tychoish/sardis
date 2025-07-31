package repo

import (
	"fmt"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
)

type Project struct {
	Name           string   `bson:"name" json:"name" yaml:"name"`
	Type           string   `bson:"type" json:"type" yaml:"type"`
	RepoName       string   `bson:"repo" json:"repo" yaml:"repo"`
	Notify         bool     `bson:"notify" json:"notify" yaml:"notify"`
	Enabled        bool     `bson:"enabled" json:"enabled" yaml:"enabled"`
	DeployCommands []string `bson:"deploy_commands" json:"deploy_commands" yaml:"deploy_commands"`
}

func (conf *Configuration) projectsValidate() error {
	set := &dt.Set[string]{}
	ec := &erc.Collector{}

	for idx := range conf.Projects {
		bc := conf.Projects[idx]
		if bc.Name == "" && bc.RepoName == "" {
			ec.Add(fmt.Errorf("blog at index %d is missing a name and a repo name", idx))
			continue
		}

		bc.Name = ft.Default(bc.Name, bc.RepoName)
		bc.RepoName = ft.Default(bc.RepoName, bc.Name)

		ec.Whenf(set.Check(bc.Name), "blog named %q(%d) has a duplicate blog configured", bc.Name, idx)
		ec.Whenf(set.Check(bc.RepoName), "blog with repo %s (named %s(%d)) has a duplicate name", bc.RepoName, bc.Name, idx)

		set.Add(bc.Name)
		set.Add(bc.RepoName)

	}

	return ec.Resolve()
}

func (conf *Configuration) ProjectsByName(name string) *Project {
	for idx := range conf.Projects {
		if conf.Projects[idx].Name == name {
			return &conf.Projects[idx]
		}

		if conf.Projects[idx].RepoName == name {
			return &conf.Projects[idx]
		}
	}
	return nil
}
