package sardis

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mongodb/grip"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings Settings      `bson:"settings" json:"settings" yaml:"settings"`
	Mail     []MailConf    `bson:"mail" json:"mail" yaml:"mail"`
	Repo     []RepoConf    `bson:"repo" json:"repo" yaml:"repo"`
	Arch     ArchLinuxConf `bson:"arch" json:"arch" yaml:"arch"`
}

type MailConf struct {
	Path   string `bson:"path" json:"path" yaml:"path"`
	Remote string `bson:"remote" json:"remote" yaml:"remote"`
	Emacs  string `bson:"emacs" json:"emacs" yaml:"emacs"`
	MuPath string `bson:"mu_path" json:"mu_path" yaml:"mu_path"`
}

type RepoConf struct {
	Path      string   `bson:"path" json:"path" yaml:"path"`
	Remote    string   `bson:"remote" json:"remote" yaml:"remote"`
	LocalSync bool     `bson:"sync" json:"sync" yaml:"sync"`
	Pre       []string `bson:"pre" json:"pre" yaml:"pre"`
	Post      []string `bson:"post" json:"post" yaml:"post"`
	Mirrors   []string `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
}

type ArchLinuxConf struct {
	BuildPath   string `bson:"build_path" json:"build_path" yaml:"build_path"`
	AurPackages []struct {
		Name   string `bson:"name" json:"name" yaml:"name"`
		Update bool   `bson:"update" json:"update" yaml:"update"`
	} `bson:"aur_packages" json:"aur_packages" yaml:"aur_packages"`
	Packages []struct {
		Name string `bson:"name" json:"name" yaml:"name"`
	} `bson:"packages" json:"packages" yaml:"packages"`
}

type NotifyConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
}

type Settings struct {
	Notification NotifyConf      `bson:"notify" json:"notify" yaml:"notify"`
	Queue        AmboyConf       `bson:"amboy" json:"amboy" yaml:"amboy"`
	Credentials  CredentialsConf `bson:"credentials" json:"credentials" yaml:"credentials"`
}

type CredentialsConf struct {
	Path string `bson:"path" json:"path" yaml:"path"`
	Jira struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
		URL      string `bson:"url" json:"url" yaml:"url"`
	} `bson:"jira" json:"jira" yaml:"jira"`
	Corp struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
		Seed     string `bson:"seed" json:"seed" yaml:"seed"`
	} `bson:"corp" json:"corp" yaml:"corp"`
	GitHub struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
		Token    string `bson:"token" json:"token" yaml:"token"`
	} `bson:"github" json:"github" yaml:"github"`
	AWS struct {
		Key    string `bson:"key" json:"key" yaml:"key"`
		Secret string `bson:"secret" json:"secret" yaml:"secret"`
		Token  string `bson:"token" json:"token" yaml:"token"`
	} `bson:"aws" json:"aws" yaml:"aws"`
	RHN struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
	} `bson:"rhn" json:"rhn" yaml:"rhn"`
}

type AmboyConf struct {
	Workers int `bson:"workers" json:"workers" yaml:"workers"`
	Size    int `bson:"size" json:"size" yaml:"size"`
}

func LoadConfiguration(fn string) (*Configuration, error) {
	out := &Configuration{}

	if err := util.UnmarshalFile(fn, out); err != nil {
		return nil, errors.Wrap(err, "problem unmarshaling config data")
	}

	return out, nil
}

type validatable interface {
	Validate() error
}

func (conf *Configuration) Validate() error {
	catcher := grip.NewBasicCatcher()
	for _, c := range []validatable{
		&conf.Settings.Notification,
		&conf.Settings.Queue,
		&conf.Settings.Credentials,
		&conf.Arch,
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

func (conf *ArchLinuxConf) Validate() error {
	if _, err := os.Stat("/etc/arch-release"); os.IsNotExist(err) {
		return nil
	}

	if conf.BuildPath == "" {
		conf.BuildPath = filepath.Join(util.GetHomeDir(), "abs")
	}

	catcher := grip.NewBasicCatcher()
	if stat, err := os.Stat(conf.BuildPath); os.IsNotExist(err) {
		catcher.Add(errors.Wrap(os.MkdirAll(conf.BuildPath, 0755), "problem making build directory"))
	} else if !stat.IsDir() {
		catcher.Add(errors.Errorf("arch build path '%s' is a file not a directory", conf.BuildPath))
	}

	for idx, pkg := range conf.AurPackages {
		if pkg.Name == "" {
			catcher.Add(errors.Errorf("aur package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			catcher.Add(errors.Errorf("aur package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}

	for idx, pkg := range conf.Packages {
		if pkg.Name == "" {
			catcher.Add(errors.Errorf("package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			catcher.Add(errors.Errorf("package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}
	return catcher.Resolve()
}

func (conf *CredentialsConf) Validate() error {
	if conf.Path == "" {
		return nil
	}
	return errors.WithStack(util.UnmarshalFile(conf.Path, &conf))
}
