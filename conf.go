package sardis

import (
	"errors"
	"fmt"
	"os"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/sysmgmt"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings   *srv.Configuration    `bson:"settings" json:"settings" yaml:"settings"`
	Repos      repo.Configuration    `bson:"repositories" json:"repositories" yaml:"repositories"`
	Operations subexec.Configuration `bson:"operations" json:"operations" yaml:"operations"`
	System     sysmgmt.Configuration `bson:"system" json:"system" yaml:"system"`

	NetworkCOMPAT  srv.Network              `bson:"network" json:"network" yaml:"network"`
	HostsCOMPAT    []srv.HostDefinition     `bson:"hosts,omitempty" json:"hosts,omitempty" yaml:"hosts,omitempty"`
	BlogCOMPAT     []repo.Project           `bson:"blog,omitempty" json:"blog,omitempty" yaml:"blog,omitempty"`
	CommandsCOMPAT []subexec.Group          `bson:"commands,omitempty" json:"commands,omitempty" yaml:"commands,omitempty"`
	RepoCOMPAT     []repo.GitRepository     `bson:"repo,omitempty" json:"repo,omitempty" yaml:"repo,omitempty"`
	LinksCOMPAT    []sysmgmt.LinkDefinition `bson:"links,omitempty" json:"links,omitempty" yaml:"links,omitempty"`

	operationsGenerated bool
	linkedFilesRead     bool
	originalPath        string

	caches struct {
		validation adt.Once[error]
	}
}

func LoadConfiguration(fn string) (*Configuration, error) {
	out, err := readConfiguration(fn)
	if err != nil {
		return nil, err
	}

	if err := out.Validate(); err != nil {
		return nil, err
	}

	return out, nil
}

func readConfiguration(fn string) (*Configuration, error) {
	if _, err := os.Stat(fn); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist [%s]", fn, err)
	}

	out := &Configuration{}

	if err := util.UnmarshalFile(fn, out); err != nil {
		return nil, fmt.Errorf("problem unmarshaling config data: %w", err)
	}
	out.originalPath = fn
	return out, nil
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	grip.Debugf("validating %q", conf.originalPath)
	conf.Settings = ft.DefaultNew(conf.Settings)

	ec := &erc.Collector{}

	ec.Push(conf.expandLinkedFiles())
	ec.Push(conf.expandOperations())

	ec.Push(conf.System.Validate())
	ec.Push(conf.Repos.Validate())
	ec.Push(conf.Operations.Validate())

	return ec.Resolve()
}

func (conf *Configuration) expandOperations() error {
	defer func() { conf.operationsGenerated = true }()

	if conf.operationsGenerated {
		return errors.New("cannot generate operations more than once")
	}

	conf.Operations.Commands = dt.MergeSlices(
		conf.Operations.Commands,
		conf.Repos.ConcreteTaskGroups(),
		conf.Repos.SyntheticTaskGroups(),
		conf.System.SystemD.TaskGroups(),
	)
	return nil
}

func (conf *Configuration) expandLinkedFiles() error {
	if conf.linkedFilesRead {
		return nil
	}
	defer func() { conf.linkedFilesRead = true }()

	conf.Migrate()
	err := fun.MakeConverterErr(func(fn string) (*Configuration, error) {
		grip.Debugf("reading linked config file %q", fn)
		iconf, err := readConfiguration(fn)
		switch {
		case err != nil:
			return nil, fmt.Errorf("problem reading linked config file %q: %w", fn, err)
		case iconf == nil:
			return nil, fmt.Errorf("nil configuration for %q", fn)
		case iconf.Settings != nil:
			return nil, fmt.Errorf("nested file %q specified system configuration", fn)
		default:
			return iconf.Migrate(), nil
		}
	}).Parallel(fun.SliceStream(conf.Settings.ConfigPaths),
		fun.WorkerGroupConfContinueOnError(),
		fun.WorkerGroupConfWorkerPerCPU(),
	).ReadAll(func(sconf *Configuration) { conf.Join(sconf) }).Wait()
	if err != nil {
		return err
	}

	return nil
}

func (conf *Configuration) Migrate() *Configuration {
	for idx := range conf.BlogCOMPAT {
		conf.BlogCOMPAT[idx].Type = "blog"
	}

	grip.Debugf("migrating config file %q", conf.originalPath)

	conf.Repos.Projects = append(conf.Repos.Projects, conf.BlogCOMPAT...)
	conf.BlogCOMPAT = nil

	conf.Operations.Commands = append(conf.Operations.Commands, conf.CommandsCOMPAT...)
	conf.CommandsCOMPAT = nil

	conf.Repos.GitRepos = append(conf.Repos.GitRepos, conf.RepoCOMPAT...)
	conf.RepoCOMPAT = nil

	conf.System.Links.Links = append(conf.System.Links.Links, conf.LinksCOMPAT...)
	conf.LinksCOMPAT = nil

	conf.NetworkCOMPAT.Hosts = append(conf.NetworkCOMPAT.Hosts, conf.NetworkCOMPAT.Hosts...)
	if conf.Settings != nil {
		conf.Settings.Network.Hosts = append(conf.Settings.Network.Hosts, conf.HostsCOMPAT...)
		conf.HostsCOMPAT = nil

		conf.Settings.Network.Hosts = append(conf.Settings.Network.Hosts, conf.NetworkCOMPAT.Hosts...)
		conf.NetworkCOMPAT.Hosts = nil
	}

	conf.System.SystemD.Services = append(conf.System.SystemD.Services, conf.System.ServicesLEGACY...)
	conf.System.ServicesLEGACY = nil

	return conf
}

func (conf *Configuration) Join(mcf *Configuration) *Configuration {
	if mcf == nil {
		return conf
	}
	grip.Debugf("merging config files: %q into %q", mcf.originalPath, conf.originalPath)

	conf.NetworkCOMPAT.Join(mcf.NetworkCOMPAT)
	conf.System.Join(mcf.System)
	conf.Repos.Join(&mcf.Repos)
	conf.Operations.Join(&mcf.Operations)
	return conf
}
