package sardis

import (
	"io/ioutil"
	"os"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/send"
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
	Name     string `bson:"name" json:"name" yaml:"name"`
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

type validatable interface {
	Validate() error
}

func (conf *Configuration) Validate() error {
	catcher := grip.NewBasicCatcher()
	for _, c := range []validatable{
		&conf.Notification,
		&conf.Queue,
	} {
		catcher.Add(errors.Wrapf(c.Validate(), "%T is not valid", c))
	}
	return catcher.Resolve()
}

func (conf *NotifyConf) Validate() error {
	if conf.Name == "" {
		conf.Name = "sardis"
	}

	if conf.Target == "" {
		conf.Target = os.Getenv("SARDIS_NOTIFY_TARGET")
	}
	defaults := send.GetXMPPConnectionInfo()
	if conf.Host == "" {
		conf.Host = defaults.Hostname
	}
	if conf.User == "" {
		conf.User = defaults.Username
	}
	if conf.Password == "" {
		conf.Password = defaults.Password
	}

	catcher := grip.NewBasicCatcher()
	for k, v := range map[string]string{
		"host": conf.Host,
		"user": conf.User,
		"pass": conf.Password,
	} {
		if v == "" {
			catcher.Add(errors.Errorf("missing value for '%s'", k))
		}
	}

	return catcher.Resolve()
}

func (conf *AmboyConf) Validate() error {
	catcher := grip.NewBasicCatcher()

	if conf.Workers < 1 {
		catcher.Add(errors.New("must specify one or more workers"))
	}

	if conf.Size < conf.Workers {
		grip.Warning("suspect config; must specify more storage than workers")
		conf.Size = 2 * conf.Workers
	}

	return catcher.Resolve()
}
