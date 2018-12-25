package sardis

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

type Configuration struct {
	Mail []MailConf `bson:"mail" json:"mail" yaml:"mail"`
}

type MailConf struct {
	Path   string `bson:"path" json:"path" yaml:"path"`
	Remote string `bson:"remote" json:"remote" yaml:"remote"`
	Emacs  string `bson:"emacs" json:"emacs" yaml:"emacs"`
	MuPath string `bson:"mu_path" json:"mu_path" yaml:"mu_path"`
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

	SetConf(&out)

	return &out, nil
}
