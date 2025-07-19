package sardis

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mitchellh/go-homedir"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/sysmgmt"
	sutil "github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings   *Settings             `bson:"settings" json:"settings" yaml:"settings"`
	Repos      repo.Configuration    `bson:"repositories" json:"repositories" yaml:"repositories"`
	Operations subexec.Configuration `bson:"operations" json:"operations" yaml:"operations"`
	Hosts      []HostConf            `bson:"hosts" json:"hosts" yaml:"hosts"`
	System     SystemConf            `bson:"system" json:"system" yaml:"system"`
	Blog       []BlogConf            `bson:"blog" json:"blog" yaml:"blog"`

	CommandsCOMPAT []subexec.Group          `bson:"commands" json:"commands" yaml:"commands"`
	RepoCOMPAT     []repo.GitRepository     `bson:"repo" json:"repo" yaml:"repo"`
	LinksCOMPAT    []sysmgmt.LinkDefinition `bson:"links" json:"links" yaml:"links"`

	generatedLocalOps bool
	linkedFilesRead   bool
	originalPath      string

	caches struct {
		validation adt.Once[error]
	}
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
	Arch           ArchLinuxConf                `bson:"arch" json:"arch" yaml:"arch"`
	SystemD        sysmgmt.SystemdConfiguration `bson:"systemd" json:"systemd" yaml:"systemd"`
	Links          sysmgmt.LinkConfiguration    `bson:"links" json:"links" yaml:"links"`
	ServicesLEGACY []sysmgmt.SystemdService     `bson:"services" json:"services" yaml:"services"`
}

type NotifyConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
	Disabled bool   `bson:"disabled" json:"disabled" yaml:"disabled"`
}

type Settings struct {
	Notification        NotifyConf       `bson:"notify" json:"notify" yaml:"notify"`
	Telegram            telegram.Options `bson:"telegram" json:"telegram" yaml:"telegram"`
	Credentials         CredentialsConf  `bson:"credentials" json:"credentials" yaml:"credentials"`
	SSHAgentSocketPath  string           `bson:"ssh_agent_socket_path" json:"ssh_agent_socket_path" yaml:"ssh_agent_socket_path"`
	AlacrittySocketPath string           `bson:"alacritty_socket_path" json:"alacritty_socket_path" yaml:"alacritty_socket_path"`
	Logging             LoggingConf      `bson:"logging" json:"logging" yaml:"logging"`
	ConfigPaths         []string         `bson:"config_files" json:"config_files" yaml:"config_files"`
	DMenuFlags          godmenu.Flags    `bson:"dmenu" json:"dmenu" yaml:"dmenu"`

	Runtime struct {
		Hostname        string `bson:"-" json:"-" yaml:"-"`
		IncludeLocalSHH bool   `bson:"include_local_ssh" json:"include_local_ssh" yaml:"include_local_ssh"`
	}
}

type LoggingConf struct {
	DisableStandardOutput     bool           `bson:"disable_standard_output" json:"disable_standard_output" yaml:"disable_standard_output"`
	EnableJSONFormating       bool           `bson:"enable_json_formatting" json:"enable_json_formatting" yaml:"enable_json_formatting"`
	EnableJSONColorFormatting bool           `bson:"enable_json_color_formatting" json:"enable_json_color_formatting" yaml:"enable_json_color_formatting"`
	DisableSyslog             bool           `bson:"disable_syslog" json:"disable_syslog" yaml:"disable_syslog"`
	Priority                  level.Priority `bson:"priority" json:"priority" yaml:"priority"`
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

type BlogConf struct {
	Name           string   `bson:"name" json:"name" yaml:"name"`
	RepoName       string   `bson:"repo" json:"repo" yaml:"repo"`
	Notify         bool     `bson:"notify" json:"notify" yaml:"notify"`
	Enabled        bool     `bson:"enabled" json:"enabled" yaml:"enabled"`
	DeployCommands []string `bson:"deploy_commands" json:"deploy_commands" yaml:"deploy_commands"`
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
	out := &Configuration{}

	if err := sutil.UnmarshalFile(fn, out); err != nil {
		return nil, fmt.Errorf("problem unmarshaling config data: %w", err)
	}
	out.originalPath = fn
	return out, nil
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	ec.Push(conf.expandLinkedFiles()) // this is where merging files (and migrating legacy files)

	conf.Settings = ft.DefaultNew(conf.Settings)
	conf.Operations.Commands = append(conf.Operations.Commands, conf.Repos.ConcreteTaskGroups()...)
	conf.Operations.Commands = append(conf.Operations.Commands, conf.Repos.SyntheticTaskGroups()...)
	conf.Operations.Commands = append(conf.Operations.Commands, conf.System.SystemD.TaskGroups()...)

	ec.Push(conf.Settings.Notification.Validate())
	ec.Push(conf.Settings.Credentials.Validate())
	ec.Push(conf.Settings.Validate())
	ec.Push(conf.System.Arch.Validate())
	ec.Push(conf.System.SystemD.Validate())
	ec.Push(conf.System.Links.Validate())
	ec.Push(conf.Repos.Validate())
	ec.Push(conf.Operations.Validate())

	for idx := range conf.Hosts {
		ec.Wrapf(conf.Hosts[idx].Validate(), "%d of %T is not valid", idx, conf.Hosts[idx])
	}

	ec.Wrap(conf.validateBlogs(), "blog build/push configuration is invalid")

	return ec.Resolve()
}

func (conf *Configuration) expandLinkedFiles() error {
	if conf.linkedFilesRead {
		return nil
	}
	defer func() { conf.linkedFilesRead = true }()

	confStream := fun.MakeConverterErr(func(fn string) (*Configuration, error) {
		if _, err := os.Stat(fn); os.IsNotExist(err) {
			return nil, fmt.Errorf("%s does not exist [%s]", fn, err)
		}

		conf, err := readConfiguration(fn)
		if err != nil {
			return nil, fmt.Errorf("problem reading linked config file %q: %w", fn, err)
		}
		if conf == nil {
			return nil, fmt.Errorf("nil configuration for %q", fn)
		}
		if conf.Settings != nil {
			return nil, fmt.Errorf("nested file %q specified system configuration", fn)

		}
		return conf, nil
	}).Parallel(fun.SliceStream(conf.Settings.ConfigPaths),
		fun.WorkerGroupConfContinueOnError(),
		fun.WorkerGroupConfWorkerPerCPU(),
	)

	confs, err := fun.NewGenerator(confStream.Slice).Wait()
	if err != nil {
		return err
	}

	if err = conf.Merge(confs...); err != nil {
		return err
	}

	return nil
}

func (conf *Configuration) Merge(mcfs ...*Configuration) error {
	ec := &erc.Collector{}

	for idx := range mcfs {
		mcf := mcfs[idx]
		if mcf == nil {
			continue
		}

		conf.LinksCOMPAT = append(conf.LinksCOMPAT, mcf.LinksCOMPAT...)
		conf.CommandsCOMPAT = append(conf.CommandsCOMPAT, mcf.CommandsCOMPAT...)
		conf.RepoCOMPAT = append(conf.RepoCOMPAT, mcf.RepoCOMPAT...)
		conf.LinksCOMPAT = append(conf.LinksCOMPAT, mcf.LinksCOMPAT...)
		conf.System.ServicesLEGACY = append(conf.System.ServicesLEGACY, mcf.System.ServicesLEGACY...)

		conf.Blog = append(conf.Blog, mcf.Blog...)
		conf.Hosts = append(conf.Hosts, mcf.Hosts...)
		conf.Operations.Commands = append(conf.Operations.Commands, mcf.Operations.Commands...)
		conf.Repos.GitRepos = append(conf.Repos.GitRepos, mcf.Repos.GitRepos...)

		conf.System.Arch.AurPackages = append(conf.System.Arch.AurPackages, mcf.System.Arch.AurPackages...)
		conf.System.Arch.Packages = append(conf.System.Arch.Packages, mcf.System.Arch.Packages...)
		conf.System.SystemD.Services = append(conf.System.SystemD.Services, mcf.System.SystemD.Services...)
		conf.System.GoPackages = append(conf.System.GoPackages, mcf.System.GoPackages...)
		conf.System.Links.Links = append(conf.System.Links.Links, mcf.System.Links.Links...)

		ec.Whenf(ft.NotZero(mcf.Operations.Settings), "config file at %q has defined operational settings that are not mergable", mcf.originalPath)
		ec.Whenf(mcf.Settings != nil, "config file at %q has defined global settings that are not mergable", mcf.originalPath)
	}

	conf.System.SystemD.Services = append(conf.System.SystemD.Services, conf.System.ServicesLEGACY...)
	conf.System.ServicesLEGACY = nil

	conf.Operations.Commands = append(conf.Operations.Commands, conf.CommandsCOMPAT...)
	conf.CommandsCOMPAT = nil

	conf.Repos.GitRepos = append(conf.Repos.GitRepos, conf.RepoCOMPAT...)
	conf.RepoCOMPAT = nil

	conf.System.Links.Links = append(conf.System.Links.Links, conf.LinksCOMPAT...)
	conf.LinksCOMPAT = nil

	return ec.Resolve()
}

func (conf *Settings) Validate() error {
	conf.Runtime.Hostname = ft.DefaultFuture(conf.Runtime.Hostname, util.GetHostname)
	conf.DMenuFlags = ft.DefaultFuture(conf.DMenuFlags, func() godmenu.Flags {
		return godmenu.Flags{
			// Path:            godmenu.DefaultDMenuPath,
			// BackgroundColor: godmenu.DefaultBackgroundColor,
			// TextColor:       godmenu.DefaultTextColor,
			// Font:            "Source Code Pro-13",
			Lines:  16,
			Prompt: "=>>",
		}
	})

	if ft.Not(conf.Telegram.IsZero()) {
		return conf.Telegram.Validate()
	}

	return nil
}

func (conf *Settings) DMenu() godmenu.Arg {
	return godmenu.WithFlags(&conf.DMenuFlags)
}

func (conf *NotifyConf) Validate() error {
	if conf == nil || (conf.Target == "" && os.Getenv("SARDIS_NOTIFY_TARGET") == "") {
		return nil
	}

	if conf.Name == "" {
		conf.Name = "sardis"
	}

	if conf.Target == "" {
		conf.Target = os.Getenv("SARDIS_NOTIFY_TARGET")
	}
	defaults := xmpp.GetConnectionInfo()
	if conf.Host == "" {
		conf.Host = defaults.Hostname
	}
	if conf.User == "" {
		conf.User = defaults.Username
	}
	if conf.Password == "" {
		conf.Password = defaults.Password
	}

	catcher := &erc.Collector{}
	for k, v := range map[string]string{
		"host": conf.Host,
		"user": conf.User,
		"pass": conf.Password,
	} {
		catcher.Whenf(v == "", "missing value for '%s'", k)
	}

	return catcher.Resolve()
}

func (conf *ArchLinuxConf) Validate() error {
	if _, err := os.Stat("/etc/arch-release"); os.IsNotExist(err) {
		return nil
	}

	if conf.BuildPath == "" {
		conf.BuildPath = filepath.Join(util.GetHomedir(), "abs")
	} else {
		var err error

		conf.BuildPath, err = homedir.Expand(conf.BuildPath)
		if err != nil {
			return err
		}
	}

	catcher := &erc.Collector{}
	if stat, err := os.Stat(conf.BuildPath); os.IsNotExist(err) {
		if err := os.MkdirAll(conf.BuildPath, 0755); err != nil {
			catcher.Add(fmt.Errorf("making %q: %w", conf.BuildPath, err))
		}
	} else if !stat.IsDir() {
		catcher.Add(fmt.Errorf("arch build path '%s' is a file not a directory", conf.BuildPath))
	}

	for idx, pkg := range conf.AurPackages {
		if pkg.Name == "" {
			catcher.Add(fmt.Errorf("aur package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			catcher.Add(fmt.Errorf("aur package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}

	for idx, pkg := range conf.Packages {
		if pkg.Name == "" {
			catcher.Add(fmt.Errorf("package at index=%d does not have name", idx))
		}
		if strings.Contains(pkg.Name, ".+=") {
			catcher.Add(fmt.Errorf("package '%s' at index=%d has invalid character", pkg.Name, idx))
		}
	}
	return catcher.Resolve()
}

func (conf *Configuration) validateBlogs() error {
	set := &dt.Set[string]{}
	ec := &erc.Collector{}

	for idx := range conf.Blog {
		bc := conf.Blog[idx]
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

func (conf *Configuration) GetBlog(name string) *BlogConf {
	for idx := range conf.Blog {
		if conf.Blog[idx].Name == name {
			return &conf.Blog[idx]
		}

		if conf.Blog[idx].RepoName == name {
			return &conf.Blog[idx]
		}
	}
	return nil
}

func (conf *CredentialsConf) Validate() error {
	if conf.Path == "" {
		return nil
	}

	var err error
	conf.Path, err = homedir.Expand(conf.Path)
	if err != nil {
		return err
	}

	return sutil.UnmarshalFile(conf.Path, &conf)
}

func (h *HostConf) Validate() error {
	catcher := &erc.Collector{}

	catcher.When(h.Name == "", ers.Error("cannot have an empty name for a host"))
	catcher.When(h.Hostname == "", ers.Error("cannot have an empty host name"))
	catcher.When(h.Port == 0, ers.Error("must specify a non-zero port number for a host"))
	catcher.When(!slices.Contains([]string{"ssh", "jasper"}, h.Protocol), ers.Error("host protocol must be ssh or jasper"))

	if h.Protocol == "ssh" {
		catcher.When(h.User == "", ers.Error("must specify user for ssh hosts"))
	}

	return catcher.Resolve()
}

func (conf *Configuration) GetHost(name string) (*HostConf, error) {
	for _, h := range conf.Hosts {
		if h.Name == name {
			return &h, nil
		}
	}

	return nil, fmt.Errorf("could not find a host named '%s'", name)
}
