package sardis

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

type Configuration struct {
	Mail         []MailConf `bson:"mail" json:"mail" yaml:"mail"`
	Repo         []RepoConf `bson:"repo" json:"repo" yaml:"repo"`
	Notification NotifyConf `bson:"notify" json:"notify" yaml:"notify"`
	Queue        AmboyConf  `bson:"amboy" json:"amboy" yaml:"amboy"`
}

type MailConf struct {
	Path   string `bson:"path" json:"path" yaml:"path"`
	Remote string `bson:"remote" json:"remote" yaml:"remote"`
	Emacs  string `bson:"emacs" json:"emacs" yaml:"emacs"`
	MuPath string `bson:"mu_path" json:"mu_path" yaml:"mu_path"`
}

type RepoConf struct {
	Path       string `bson:"path" json:"path" yaml:"path"`
	Remote     string `bson:"remote" json:"remote" yaml:"remote"`
	ShouldSync bool   `bson:"sync" json:"sync" yaml:"sync"`
}

type NotifyConf struct {
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
}

type AmboyConf struct {
	Workers int `bson:"workers" json:"workers" yaml:"workers"`
	Size    int `bson:"size" json:"size" yaml:"size"`
}

func LoadConfiguration(fn string) (*Configuration, error) {
	if stat, err := os.Stat(fn); os.IsNotExist(err) || stat.IsDir() {
		return nil, errors.Errorf("'%s' does not exist", fn)
	}

	unmarshal := getUnmarshaler(fn)
	if unmarshal == nil {
		return nil, errors.Errorf("cannot find unmarshler for input %s", fn)
	}

	file, err := os.Open(fn)
	if err != nil {
		return nil, errors.Wrapf(err, "problem opening file '%s'", fn)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrapf(err, "problem reading data from %s", fn)
	}

	out := Configuration{}
	if err = unmarshal(data, &out); err != nil {
		return nil, errors.Wrap(err, "problem unmarshaling report data")
	}

	return &out, nil
}
