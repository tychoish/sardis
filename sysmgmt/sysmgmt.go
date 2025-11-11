package sysmgmt

import (
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
)

type Configuration struct {
	GoPackages []GoPackage          `bson:"golang" json:"golang" yaml:"golang"`
	Arch       ArchLinux            `bson:"arch" json:"arch" yaml:"arch"`
	SystemD    SystemdConfiguration `bson:"systemd" json:"systemd" yaml:"systemd"`
	Links      LinkConfiguration    `bson:"links" json:"links" yaml:"links"`

	ServicesLEGACY []SystemdService `bson:"services,omitempty" json:"services,omitempty" yaml:"services,omitempty"`
}

func (conf *Configuration) Validate() error {
	return erc.Join(
		conf.Arch.Validate(),
		conf.SystemD.Validate(),
		conf.Links.Validate(),
	)
}

func (conf *Configuration) Join(mcf Configuration) {
	if mcf.Links.Discovery != nil {
		conf.Links.Discovery = ft.DefaultNew(conf.Links.Discovery)
		conf.Links.Discovery.SearchPaths = append(conf.Links.Discovery.SearchPaths, mcf.Links.Discovery.SearchPaths...)
		conf.Links.Discovery.IgnoreTargetPrefixes = append(conf.Links.Discovery.IgnoreTargetPrefixes, mcf.Links.Discovery.IgnoreTargetPrefixes...)
		conf.Links.Discovery.IgnorePathPrefixes = append(conf.Links.Discovery.IgnorePathPrefixes, mcf.Links.Discovery.IgnorePathPrefixes...)
		conf.Links.Discovery.SkipResolvedTargets = ft.Default(mcf.Links.Discovery.SkipResolvedTargets, conf.Links.Discovery.SkipResolvedTargets)
		conf.Links.Discovery.SkipMissingTargets = ft.Default(mcf.Links.Discovery.SkipMissingTargets, conf.Links.Discovery.SkipMissingTargets)
	}

	conf.Arch.Packages = append(conf.Arch.Packages, mcf.Arch.Packages...)
	conf.GoPackages = append(conf.GoPackages, mcf.GoPackages...)
	conf.Links.Links = append(conf.Links.Links, mcf.Links.Links...)
	conf.SystemD.Services = append(conf.SystemD.Services, mcf.SystemD.Services...)
}
