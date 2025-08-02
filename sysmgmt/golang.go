package sysmgmt

type GoPackage struct {
	Name    string `bson:"name" json:"name" yaml:"name"`
	Update  bool   `bson:"update" json:"update" yaml:"update"`
	Version string `bson:"version" json:"version" yaml:"version"`
}
