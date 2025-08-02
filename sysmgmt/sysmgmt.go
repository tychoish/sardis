package sysmgmt

import "github.com/tychoish/fun/erc"

type Configuration struct {
	GoPackages     []GoPackage          `bson:"golang" json:"golang" yaml:"golang"`
	Arch           ArchLinux            `bson:"arch" json:"arch" yaml:"arch"`
	SystemD        SystemdConfiguration `bson:"systemd" json:"systemd" yaml:"systemd"`
	Links          LinkConfiguration    `bson:"links" json:"links" yaml:"links"`
	ServicesLEGACY []SystemdService     `bson:"services,omitempty" json:"services,omitempty" yaml:"services,omitempty"`
}

func (conf *Configuration) Validate() error {
	return erc.Join(
		conf.Arch.Validate(),
		conf.SystemD.Validate(),
		conf.Links.Validate(),
	)
}

func (conf *Configuration) Join(mcf Configuration) {
	conf.Arch.AurPackages = append(conf.Arch.AurPackages, mcf.Arch.AurPackages...)
	conf.Arch.Packages = append(conf.Arch.Packages, mcf.Arch.Packages...)
	conf.GoPackages = append(conf.GoPackages, mcf.GoPackages...)
	conf.Links.Links = append(conf.Links.Links, mcf.Links.Links...)
	conf.SystemD.Services = append(conf.SystemD.Services, mcf.SystemD.Services...)
	conf.SystemD.Services = append(conf.SystemD.Services, mcf.ServicesLEGACY...)
	conf.ServicesLEGACY = nil
}
