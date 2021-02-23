package sardis

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/deciduosity/utility"
	git "github.com/go-git/go-git/v5"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings Settings      `bson:"settings" json:"settings" yaml:"settings"`
	Mail     []MailConf    `bson:"mail" json:"mail" yaml:"mail"`
	Repo     []RepoConf    `bson:"repo" json:"repo" yaml:"repo"`
	Links    []LinkConf    `bson:"links" json:"links" yaml:"links"`
	Hosts    []HostConf    `bson:"hosts" json:"hosts" yaml:"hosts"`
	System   SystemConf    `bson:"system" json:"system" yaml:"system"`
	Commands []CommandConf `bson:"commands" json:"commands" yaml:"commands"`
	Projects []ProjectConf `bson:"projects" json:"projects" yaml:"projects"`
	Blog     []BlogConf    `bson:"blog" json:"blog" yaml:"blog"`
}

type MailConf struct {
	Name    string   `bson:"name" json:"name" yaml:"name"`
	Path    string   `bson:"path" json:"path" yaml:"path"`
	Remote  string   `bson:"remote" json:"remote" yaml:"remote"`
	Emacs   string   `bson:"emacs" json:"emacs" yaml:"emacs"`
	MuPath  string   `bson:"mu_path" json:"mu_path" yaml:"mu_path"`
	Mirrors []string `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
	Pre     []string `bson:"pre" json:"pre" yaml:"pre"`
	Post    []string `bson:"post" json:"post" yaml:"post"`
}

type RepoConf struct {
	Name       string   `bson:"name" json:"name" yaml:"name"`
	Path       string   `bson:"path" json:"path" yaml:"path"`
	Remote     string   `bson:"remote" json:"remote" yaml:"remote"`
	RemoteName string   `bson:"remote_name" json:"remote_name" yaml:"remote_name"`
	Branch     string   `bson:"branch" json:"branch" yaml:"branch"`
	LocalSync  bool     `bson:"sync" json:"sync" yaml:"sync"`
	Fetch      bool     `bson:"fetch" json:"fetch" yaml:"fetch"`
	Pre        []string `bson:"pre" json:"pre" yaml:"pre"`
	Post       []string `bson:"post" json:"post" yaml:"post"`
	Mirrors    []string `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
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

type SystemConf struct {
	GoPackages []struct {
		Name    string `bson:"name" json:"name" yaml:"name"`
		Update  bool   `bson:"update" json:"update" yaml:"update"`
		Version string `bson:"version" json:"version" yaml:"version"`
	} `bson:"golang" json:"golang" yaml:"golang"`
	Arch ArchLinuxConf `bson:"arch" json:"arch" yaml:"arch"`
}

type NotifyConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
}

type LinkConf struct {
	Name              string `bson:"name" json:"name" yaml:"name"`
	Path              string `bson:"path" json:"path" yaml:"path"`
	Target            string `bson:"target" json:"target" yaml:"target"`
	Update            bool   `bson:"update" json:"update" yaml:"update"`
	DirectoryContents bool   `bson:"directory_contents" json:"directory_contents" yaml:"directory_contents"`
}

type Settings struct {
	Notification       NotifyConf      `bson:"notify" json:"notify" yaml:"notify"`
	Queue              AmboyConf       `bson:"amboy" json:"amboy" yaml:"amboy"`
	Credentials        CredentialsConf `bson:"credentials" json:"credentials" yaml:"credentials"`
	SSHAgentSocketPath string          `bson:"ssh_agent_socket_path" json:"ssh_agent_socket_path" yaml:"ssh_agent_socket_path"`
	Logging            LoggingConf     `bson:"logging" json:"logging" yaml:"logging"`
}

type LoggingConf struct {
	Name                  string         `bson:"name" json:"name" yaml:"name"`
	DisableStandardOutput bool           `bson:"disable_standard_output" json:"disable_standard_output" yaml:"disable_standard_output"`
	EnableJSONFormating   bool           `bson:"enable_json_formatting" json:"enable_json_formatting" yaml:"enable_json_formatting"`
	Priority              level.Priority `bson:"priority" json:"priority" yaml:"priority"`
}

type CredentialsConf struct {
	Path    string `bson:"path" json:"path" yaml:"path"`
	Twitter struct {
		Username       string `bson:"username" json:"username" yaml:"username"`
		ConsumerKey    string `bson:"consumer_key" json:"consumer_key" yaml:"consumer_key"`
		ConsumerSecret string `bson:"consumer_secret" json:"consumer_secret" yaml:"consumer_secret"`
		OauthToken     string `bson:"oauth_token" json:"oauth_token" yaml:"oauth_token"`
		OauthSecret    string `bson:"oauth_secret" json:"oauth_secret" yaml:"oauth_secret"`
	} `bson:"twitter" json:"twitter" yaml:"twitter"`
	Jira struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
		URL      string `bson:"url" json:"url" yaml:"url"`
	} `bson:"jira" json:"jira" yaml:"jira"`
	GitHub struct {
		Username string `bson:"username" json:"username" yaml:"username"`
		Password string `bson:"password" json:"password" yaml:"password"`
		Token    string `bson:"token" json:"token" yaml:"token"`
	} `bson:"github" json:"github" yaml:"github"`
	AWS []CredentialsAWS `bson:"aws" json:"aws" yaml:"aws"`
}

type CredentialsAWS struct {
	Profile string `bson:"profile" json:"profile" yaml:"profile"`
	Key     string `bson:"key" json:"key" yaml:"key"`
	Secret  string `bson:"secret" json:"secret" yaml:"secret"`
	Token   string `bson:"token" json:"token" yaml:"token"`
}

type AmboyConf struct {
	Workers int `bson:"workers" json:"workers" yaml:"workers"`
	Size    int `bson:"size" json:"size" yaml:"size"`
}

type CommandConf struct {
	Name      string `bson:"name" json:"name" yaml:"name"`
	Directory string `bson:"directory" json:"directory" yaml:"directory"`
	Command   string `bson:"command" json:"command" yaml:"command"`
}

type ProjectConf struct {
	Name         string                `bson:"name" json:"name" yaml:"name"`
	Options      ProjectOptions        `bson:"options" json:"options" yaml:"options"`
	Repositories []ProjectRepositories `bson:"repos" json:"repos" yaml:"repos"`
}

type BlogConf struct {
	RepoName       string   `bson:"repo" json:"repo" yaml:"repo"`
	Notify         bool     `bson:"notifyt" json:"notifyt" yaml:"notifyt"`
	Enabled        bool     `bson:"enabled" json:"enabled" yaml:"enabled"`
	DeployCommands []string `bson:"deploy_commands" json:"deploy_commands" yaml:"deploy_commands"`
}

type ProjectOptions struct {
	GithubOrg string `bson:"github_org" json:"github_org" yaml:"github_org"`
	Directory string `bson:"directory" json:"directory" yaml:"directory"`
}

type ProjectRepositories struct {
	Name string `bson:"name" json:"name" yaml:"name"`
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

	catcher.Add(conf.Settings.Validate())
	catcher.Add(conf.System.Arch.Validate())

	for idx := range conf.Repo {
		catcher.Wrapf(conf.Repo[idx].Validate(), "%d of %T is not valid", idx, conf.Repo[idx])
	}

	for idx := range conf.Mail {
		catcher.Wrapf(conf.Mail[idx].Validate(), "%d of %T is not valid", idx, conf.Mail[idx])
	}

	for idx := range conf.Projects {
		catcher.Wrapf(conf.Projects[idx].Validate(), "%d of %T is not valid", idx, conf.Projects[idx])
	}

	conf.Links = conf.expandLinks(catcher)
	for idx := range conf.Links {
		catcher.Wrapf(conf.Links[idx].Validate(), "%d of %T is not valid", idx, conf.Links[idx])
	}

	for idx := range conf.Hosts {
		catcher.Wrapf(conf.Hosts[idx].Validate(), "%d of %T is not valid", idx, conf.Hosts[idx])
	}

	for idx := range conf.Commands {
		catcher.Wrapf(conf.Commands[idx].Validate(), "%d of %T is not valid", idx, conf.Commands[idx])
	}

	return catcher.Resolve()
}

func (conf *Configuration) expandLinks(catcher grip.Catcher) []LinkConf {
	var err error
	links := make([]LinkConf, 0, len(conf.Links))
	for idx := range conf.Links {
		lnk := conf.Links[idx]
		lnk.Target, err = homedir.Expand(lnk.Target)
		if err != nil {
			catcher.Add(err)
			continue
		}

		lnk.Path, err = homedir.Expand(lnk.Path)
		if err != nil {
			catcher.Add(err)
			continue
		}

		if lnk.DirectoryContents {
			files, err := ioutil.ReadDir(lnk.Target)
			if err != nil {
				catcher.Add(err)
				continue
			}
			for _, info := range files {
				name := info.Name()
				links = append(links, LinkConf{
					Name:   fmt.Sprintf("%s:%s", lnk.Name, name),
					Path:   filepath.Join(lnk.Path, name),
					Target: filepath.Join(lnk.Target, name),
					Update: lnk.Update,
				})
			}
		} else {
			links = append(links, lnk)
		}
	}

	return links
}

func (conf *Settings) Validate() error {
	catcher := grip.NewBasicCatcher()
	for _, c := range []validatable{
		&conf.Notification,
		&conf.Queue,
		&conf.Credentials,
	} {
		catcher.Wrapf(c.Validate(), "%T is not valid", c)
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
	} else {
		var err error

		conf.BuildPath, err = homedir.Expand(conf.BuildPath)
		if err != nil {
			return errors.WithStack(err)
		}
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

func (conf *RepoConf) Validate() error {
	if conf.Branch == "" {
		conf.Branch = "master"
	}

	if conf.RemoteName == "" {
		conf.RemoteName = "origin"
	}

	if conf.Remote == "" {
		return errors.Errorf("'%s' does not specify a remote", conf.Name)
	}

	if conf.Fetch && conf.LocalSync {
		return errors.New("cannot specify sync and fetch")
	}

	var err error
	conf.Path, err = homedir.Expand(conf.Path)
	if err != nil {
		return errors.WithStack(err)
	}

	conf.Post = util.TryExpandHomeDirs(conf.Post)
	conf.Pre = util.TryExpandHomeDirs(conf.Pre)

	return nil
}

func (conf *Configuration) GetRepo(name string) *RepoConf {
	for idx := range conf.Repo {
		if conf.Repo[idx].Name == name {
			return &conf.Repo[idx]

		}
	}
	return nil
}

func (conf *RepoConf) HasChanges() (bool, error) {
	if !utility.FileExists(conf.Path) {
		return true, nil
	}

	repo, err := git.PlainOpen(conf.Path)
	if err != nil {
		return false, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	stat, err := wt.Status()
	if err != nil {
		return false, err
	}

	return !stat.IsClean(), nil
}

func (conf *LinkConf) Validate() error {
	if conf.Target == "" {
		return errors.New("must specify a link target")
	}

	if conf.Name == "" {
		fn := filepath.Dir(conf.Path)

		if fn != "" {
			conf.Name = fn
		} else {
			return errors.New("must specify a name for the link")
		}

		return errors.New("must specify a name")
	}

	if conf.Path == "" {
		base := filepath.Base(conf.Name)
		fn := filepath.Dir(conf.Name)
		if base != "" && fn != "" {
			conf.Path = base
			conf.Name = fn
		} else {
			return errors.New("must specify a location for the link")
		}
	}

	return nil
}

func (conf *MailConf) Validate() error {
	var err error
	conf.Path, err = homedir.Expand(conf.Path)
	if err != nil {
		return errors.WithStack(err)
	}

	conf.MuPath, err = homedir.Expand(conf.MuPath)
	if err != nil {
		return errors.WithStack(err)
	}

	conf.Post = util.TryExpandHomeDirs(conf.Post)
	conf.Pre = util.TryExpandHomeDirs(conf.Pre)

	return nil
}

func (conf *CredentialsConf) Validate() error {
	if conf.Path == "" {
		return nil
	}

	var err error
	conf.Path, err = homedir.Expand(conf.Path)
	if err != nil {
		return errors.WithStack(err)
	}

	return errors.WithStack(util.UnmarshalFile(conf.Path, &conf))
}

func (h *HostConf) Validate() error {
	catcher := grip.NewBasicCatcher()

	catcher.NewWhen(h.Name == "", "cannot have an empty name for a host")
	catcher.NewWhen(h.Hostname == "", "cannot have an empty host name")
	catcher.NewWhen(h.Port == 0, "must specify a non-zero port number for a host")
	catcher.NewWhen(!util.StringSliceContains([]string{"ssh", "jasper"}, h.Protocol), "host protocol must be ssh or jasper")

	if h.Protocol == "ssh" {
		catcher.NewWhen(h.User == "", "must specify user for ssh hosts")
	}

	return catcher.Resolve()
}

func (conf *Configuration) GetHost(name string) (*HostConf, error) {
	for _, h := range conf.Hosts {
		if h.Name == name {
			return &h, nil
		}
	}

	return nil, errors.Errorf("could not find a host named '%s'", name)
}

func (conf *ProjectConf) Validate() error {
	var err error
	conf.Options.Directory, err = homedir.Expand(conf.Options.Directory)
	return err
}

func (conf *CommandConf) Validate() error {
	catcher := grip.NewBasicCatcher()
	catcher.NewWhen(conf.Name == "", "commands must have a name")
	catcher.NewWhen(conf.Command == "", "commands must have specify commands")

	var err error
	conf.Directory, err = homedir.Expand(conf.Directory)
	catcher.Add(err)

	return catcher.Resolve()
}

func (conf *Configuration) ExportCommands() map[string]CommandConf {
	out := make(map[string]CommandConf, len(conf.Commands))
	for idx := range conf.Commands {
		cmd := conf.Commands[idx]
		out[cmd.Name] = cmd
	}

	return out
}
