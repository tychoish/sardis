package sardis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/mitchellh/go-homedir"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	sutil "github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings   *Settings             `bson:"settings" json:"settings" yaml:"settings"`
	Repo       []repo.Configuration  `bson:"repo" json:"repo" yaml:"repo"`
	Links      []LinkConf            `bson:"links" json:"links" yaml:"links"`
	Hosts      []HostConf            `bson:"hosts" json:"hosts" yaml:"hosts"`
	System     SystemConf            `bson:"system" json:"system" yaml:"system"`
	Operations subexec.Configuration `bson:"operations" json:"operations" yaml:"operations"`
	Blog       []BlogConf            `bson:"blog" json:"blog" yaml:"blog"`
	Menus      []MenuConf            `bson:"menu" json:"menu" yaml:"menu"`

	CommandsCOMPAT []subexec.Group `bson:"commands" json:"commands" yaml:"commands"`

	generatedLocalOps bool
	repoTagsEvaluated bool
	linkedFilesRead   bool
	originalPath      string

	repoTags dt.Map[string, dt.Slice[*repo.Configuration]]
	caches   struct {
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
	Arch     ArchLinuxConf        `bson:"arch" json:"arch" yaml:"arch"`
	Services []SystemdServiceConf `bson:"services" json:"services" yaml:"services"`
}

type SystemdServiceConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Unit     string `bson:"unit" json:"unit" yaml:"unit"`
	User     bool   `bson:"user" json:"user" yaml:"user"`
	System   bool   `bson:"system" json:"system" yaml:"system"`
	Enabled  bool   `bson:"enabled" json:"enabled" yaml:"enabled"`
	Disabled bool   `bson:"disabled" json:"disabled" yaml:"disabled"`
	Start    bool   `bson:"start" json:"start" yaml:"start"`
}

func (c *SystemdServiceConf) Validate() error {
	catcher := &erc.Collector{}
	catcher.When(c.Name == "", ers.Error("must specify service name"))
	catcher.Whenf(c.Unit == "", "cannot specify empty unit for %q", c.Name)
	catcher.Whenf((c.User && c.System) || (!c.User && !c.System),
		"must specify either user or service for %q", c.Name)
	catcher.Whenf((c.Disabled && c.Enabled) || (!c.Disabled && !c.Enabled),
		"must specify either disabled or enabled for %q", c.Name)
	return catcher.Resolve()
}

type NotifyConf struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
	Disabled bool   `bson:"disabled" json:"disabled" yaml:"disabled"`
}

type LinkConf struct {
	Name              string `bson:"name" json:"name" yaml:"name"`
	Path              string `bson:"path" json:"path" yaml:"path"`
	Target            string `bson:"target" json:"target" yaml:"target"`
	Update            bool   `bson:"update" json:"update" yaml:"update"`
	DirectoryContents bool   `bson:"directory_contents" json:"directory_contents" yaml:"directory_contents"`
	RequireSudo       bool   `bson:"sudo" json:"sudo" yaml:"sudo"`
}

type Settings struct {
	Notification        NotifyConf       `bson:"notify" json:"notify" yaml:"notify"`
	Telegram            telegram.Options `bson:"telegram" json:"telegram" yaml:"telegram"`
	Credentials         CredentialsConf  `bson:"credentials" json:"credentials" yaml:"credentials"`
	SSHAgentSocketPath  string           `bson:"ssh_agent_socket_path" json:"ssh_agent_socket_path" yaml:"ssh_agent_socket_path"`
	AlacrittySocketPath string           `bson:"alacritty_socket_path" json:"alacritty_socket_path" yaml:"alacritty_socket_path"`
	Logging             LoggingConf      `bson:"logging" json:"logging" yaml:"logging"`
	ConfigPaths         []string         `bson:"config_files" json:"config_files" yaml:"config_files"`
	DMenu               godmenu.Flags    `bson:"dmenu" json:"dmenu" yaml:"dmenu"`

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

type MenuConf struct {
	Name       string   `bson:"name" json:"name" yaml:"name"`
	Command    string   `bson:"command" json:"command" yaml:"command"`
	Selections []string `bson:"selections" json:"selections" yaml:"selections"`
	Notify     bool     `bson:"notify" json:"notify" yaml:"notify"`
	Background bool     `bson:"background" json:"background" yaml:"background"`
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

func (conf *MenuConf) Validate() error {
	ec := &erc.Collector{}

	if conf.Command != "" {
		base := strings.Split(conf.Command, " ")[0]
		_, err := exec.LookPath(base)
		ec.Add(ers.Wrapf(err, "%s [%s] does not exist", base, conf.Command))
	}

	ec.Whenf(len(conf.Selections) == 0, "must specify options for %q", conf.Name)

	return ec.Resolve()
}

func (conf *Configuration) Validate() error { return conf.caches.validation.Call(conf.doValidate) }
func (conf *Configuration) doValidate() error {
	ec := &erc.Collector{}
	conf.Settings = ft.DefaultNew(conf.Settings)

	ec.Push(conf.Settings.Validate())
	ec.Push(conf.System.Arch.Validate())
	ec.Push(conf.expandLinkedFiles())

	for idx := range conf.System.Services {
		ec.Wrapf(conf.System.Services[idx].Validate(), "%d of %T is not valid", idx, len(conf.System.Services), conf.System.Services[idx])
	}

	for idx := range conf.Repo {
		ec.Wrapf(conf.Repo[idx].Validate(), "%d/%d of %T is not valid", idx, len(conf.Repo), conf.Repo[idx])
	}

	for idx := range conf.Menus {
		ec.Whenf(conf.Menus[idx].Name == "", "must specify name for menu spec at item %d", idx)
		ec.Wrapf(conf.Menus[idx].Validate(), "%d of %T is not valid", idx, conf.Menus[idx])
	}

	conf.Links = conf.expandLinks(ec)
	for idx := range conf.Links {
		ec.Wrapf(conf.Links[idx].Validate(), "%d/%d of %T is not valid", idx, len(conf.Links), conf.Links[idx])
		conf.Links[idx].Target = strings.ReplaceAll(conf.Links[idx].Target, "{{hostname}}", conf.Operations.Settings.Hostname)
	}

	for idx := range conf.Hosts {
		ec.Wrapf(conf.Hosts[idx].Validate(), "%d of %T is not valid", idx, conf.Hosts[idx])
	}

	conf.mapReposByTags()
	conf.expandLocalNativeOps()
	ec.Push(conf.Operations.Validate())

	ec.Wrap(conf.validateBlogs(), "blog build/push configuration is invalid")

	return ec.Resolve()
}

func (conf *Configuration) expandLocalNativeOps() {
	if conf.generatedLocalOps {
		return
	}
	defer func() { conf.generatedLocalOps = true }()

	grps := make([]subexec.Group, 0, len(conf.Repo)+len(conf.repoTags)+len(conf.System.Services)+len(conf.Menus))
	for idx := range conf.Repo {
		repo := conf.Repo[idx]
		if repo.Disabled {
			continue
		}
		if !repo.Fetch && !repo.LocalSync {
			continue
		}

		cg := subexec.Group{
			Name:          "repo",
			Notify:        ft.Ptr(repo.Notify),
			CmdNamePrefix: repo.Name,
			Commands: []subexec.Command{
				subexec.Command{
					Name:             "pull",
					WorkerDefinition: repo.FetchJob(),
				},
				// TODO figure out why this err doesn't really work
				//      - implementation problem with the underlying operation
				//      - also need to wrap it in a pager at some level.
				//      - and disable syslogging
				// {
				// 	Name:            "status",
				// 	Directory:       repo.Path,
				// 	OverrideDefault: true,
				// 	Command:         "alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command sardis repo status {{prefix}}",
				// },
			},
		}

		if repo.LocalSync {
			cg.Commands = append(cg.Commands, subexec.Command{
				Name:             "update",
				WorkerDefinition: repo.UpdateJob(),
			})
		}

		grps = append(grps, cg)
	}

	for tag, repos := range conf.repoTags {
		if len(repos) == 0 {
			continue
		}
		if len(repos) == 1 && repos[0].Name == tag {
			continue
		}
		anyActive := false
		for _, r := range repos {
			if r.Disabled || (r.Fetch == false && r.LocalSync == false) {
				continue
			}
			anyActive = true
			break
		}
		if !anyActive {
			continue
		}

		tagName := tag

		grps = append(grps, subexec.Group{
			Name:          "repo",
			Notify:        ft.Ptr(true),
			CmdNamePrefix: fmt.Sprint("tagged.", tagName),
			Commands: []subexec.Command{
				{
					Name: "pull",
					WorkerDefinition: func(ctx context.Context) error {
						repos := fun.MakeConverter(func(r *repo.Configuration) fun.Worker {
							return r.FetchJob()
						}).Stream(conf.repoTags.Get(tagName).Stream())

						return repos.Parallel(
							func(ctx context.Context, op fun.Worker) error { return op(ctx) },
							fun.WorkerGroupConfContinueOnError(),
							fun.WorkerGroupConfWorkerPerCPU(),
						).Run(ctx)
					},
				},
				{
					Name: "update",
					WorkerDefinition: func(ctx context.Context) error {
						repos := fun.MakeConverter(func(r *repo.Configuration) fun.Worker {
							return r.UpdateJob()
						}).Stream(conf.repoTags.Get(tagName).Stream())

						return repos.Parallel(
							func(ctx context.Context, op fun.Worker) error { return op(ctx) },
							fun.WorkerGroupConfContinueOnError(),
							fun.WorkerGroupConfWorkerPerCPU(),
						).Run(ctx)
					},
				},
			},
		})
	}

	for _, service := range conf.System.Services {
		var command string
		if service.User {
			command = "systemctl --user"
		} else {
			command = "sudo systemctl"
		}

		var defaultState string
		if service.Enabled && !service.Disabled {
			defaultState = "enable"
		} else {
			defaultState = "disable"
		}

		// TODO these have a fun.Worker function already
		// implemented in units for setup.
		grps = append(grps, subexec.Group{
			Name:          "systemd",
			Directory:     conf.Operations.Settings.Hostname,
			Notify:        ft.Ptr(true),
			CmdNamePrefix: fmt.Sprint("service." + service.Name),
			Command:       fmt.Sprintf("%s {{name}} %s", command, service.Unit),
			Commands: []subexec.Command{
				{Name: "restart"},
				{Name: "stop"},
				{Name: "start"},
				{Name: "enable"},
				{Name: "disable"},
				{Name: "setup", Command: defaultState},
				{
					Name:            "logs",
					Command:         fmt.Sprintf("alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command journalctl --follow --pager-end --unit %s", service.Unit),
					OverrideDefault: true,
				},
				{
					Name:            "status",
					Command:         fmt.Sprintf("alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command %s {{name}} %s", command, service.Unit),
					OverrideDefault: true,
				},
			},
		})
	}
	for _, menus := range conf.Menus {
		cmdGroup := subexec.Group{
			Name:       menus.Name,
			Command:    fmt.Sprintf("%s {{command}}", menus.Command),
			Notify:     ft.Ptr(menus.Notify),
			Background: ft.Ptr(menus.Background),
		}
		for _, operation := range menus.Selections {
			cmdGroup.Commands = append(cmdGroup.Commands, subexec.Command{Name: operation, Command: operation})
		}
		grps = append(grps, cmdGroup)
	}

	conf.Operations.Commands = append(conf.Operations.Commands, grps...)
}

func (conf *Configuration) expandLinkedFiles() error {
	if conf.linkedFilesRead {
		return nil
	}
	defer func() { conf.linkedFilesRead = true }()

	ec := &erc.Collector{}
	pipe := make(chan *Configuration, len(conf.Settings.ConfigPaths))

	wg := &sync.WaitGroup{}
	for idx, fileName := range conf.Settings.ConfigPaths {
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			grip.Warning(message.Fields{
				"file": fileName,
				"msg":  "config file does not exist",
			})
			continue
		}

		wg.Add(1)
		go func(idx int, fn string) {
			defer wg.Done()
			conf, err := readConfiguration(fn)
			if err != nil {
				ec.Push(fmt.Errorf("problem reading linked config file %q: %w", fn, err))
				return
			}
			if conf == nil {
				ec.Add(fmt.Errorf("nil configuration for %q", fn))
				return
			}

			ec.Whenf(conf.Settings != nil, "nested file %q specified system configuration", fn)

			pipe <- conf
		}(idx, fileName)
	}

	wg.Wait()
	close(pipe)

	confs := make([]*Configuration, 0, len(conf.Settings.ConfigPaths))
	for c := range pipe {
		confs = append(confs, c)
	}

	ec.Push(conf.Merge(confs...))
	return ec.Resolve()
}

func (conf *Configuration) Merge(mcfs ...*Configuration) error {
	mcfs = append([]*Configuration{}, mcfs...)
	reposAdded := 0
	ec := &erc.Collector{}

	for idx := range mcfs {
		mcf := mcfs[idx]
		if mcf == nil {
			continue
		}

		reposAdded += len(mcf.Repo)
		conf.Blog = append(conf.Blog, mcf.Blog...)
		conf.CommandsCOMPAT = append(conf.CommandsCOMPAT, mcf.CommandsCOMPAT...)
		conf.Operations.Commands = append(conf.Operations.Commands, mcf.Operations.Commands...)
		conf.Hosts = append(conf.Hosts, mcf.Hosts...)
		conf.Links = append(conf.Links, mcf.Links...)
		conf.Menus = append(conf.Menus, mcf.Menus...)
		conf.Repo = append(conf.Repo, mcf.Repo...)
		conf.System.Arch.AurPackages = append(conf.System.Arch.AurPackages, mcf.System.Arch.AurPackages...)
		conf.System.Arch.Packages = append(conf.System.Arch.Packages, mcf.System.Arch.Packages...)
		conf.System.GoPackages = append(conf.System.GoPackages, mcf.System.GoPackages...)
		conf.System.Services = append(conf.System.Services, mcf.System.Services...)

		ec.Whenf(ft.NotZero(mcf.Operations.Settings), "config file at %q has defined operational settings that are not mergable", mcf.originalPath)
		ec.Whenf(mcf.Settings != nil, "config file at %q has defined global settings that are not mergable", mcf.originalPath)
	}
	conf.Operations.Commands = append(conf.Operations.Commands, conf.CommandsCOMPAT...)
	conf.CommandsCOMPAT = nil

	if reposAdded > 0 {
		conf.repoTagsEvaluated = false
	}
	return ec.Resolve()
}

func (conf *Configuration) expandLinks(catcher *erc.Collector) []LinkConf {
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
			files, err := os.ReadDir(lnk.Target)
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

func (conf *Configuration) GetTaggedRepos(tags ...string) dt.Slice[repo.Configuration] {
	if len(tags) == 0 && len(conf.repoTags) == 0 {
		return nil
	}

	var out []repo.Configuration

	for _, t := range tags {
		rs, ok := conf.repoTags[t]
		if !ok {
			continue
		}

		for idx := range rs {
			out = append(out, *rs[idx])
		}
	}

	return out
}

func (conf *Configuration) RepoTags() dt.Map[string, dt.Slice[*repo.Configuration]] {
	conf.mapReposByTags()
	return conf.repoTags
}

func (conf *Configuration) mapReposByTags() {
	defer func() { conf.repoTagsEvaluated = true }()
	if conf.repoTagsEvaluated {
		return
	}

	conf.repoTags = make(map[string]dt.Slice[*repo.Configuration])

	for idx := range conf.Repo {
		for _, tag := range conf.Repo[idx].Tags {
			rp := conf.Repo[idx]
			conf.repoTags[tag] = append(conf.repoTags[tag], &rp)
		}

		repoName := conf.Repo[idx].Name

		rp := conf.Repo[idx]
		conf.repoTags[repoName] = append(
			conf.repoTags[repoName],
			&rp,
		)
	}
}

func (conf *Settings) Validate() error {
	catcher := &erc.Collector{}
	for _, c := range []interface{ Validate() error }{
		&conf.Notification,
		&conf.Credentials,
		&conf.Telegram,
	} {
		if z, ok := c.(interface{ IsZero() bool }); ok && z.IsZero() {
			continue
		}

		catcher.Wrapf(c.Validate(), "%T is not valid", c)
	}

	conf.DMenu = ft.DefaultFuture(conf.DMenu, defaultDMenuConf)
	conf.Runtime.Hostname = makeErrorHandler[string](catcher.Push)(os.Hostname())

	return catcher.Resolve()
}

func makeErrorHandler[T any](eh func(error)) func(T, error) T {
	return func(v T, err error) T { eh(err); return v }
}

func defaultDMenuConf() godmenu.Flags {
	return godmenu.Flags{
		Path:            godmenu.DefaultDMenuPath,
		BackgroundColor: godmenu.DefaultBackgroundColor,
		TextColor:       godmenu.DefaultTextColor,
		Font:            "Source Code Pro-13",
		Lines:           16,
		Prompt:          "=>>",
	}
}

func (conf *NotifyConf) IsZero() bool {
	if conf == nil || (conf.Target == "" && os.Getenv("SARDIS_NOTIFY_TARGET") == "") {
		return true
	}
	return false
}

func (conf *NotifyConf) Validate() error {
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

func (conf *Configuration) GetRepo(name string) *repo.Configuration {
	for idx := range conf.Repo {
		if conf.Repo[idx].Name == name {
			return &conf.Repo[idx]
		}

	}
	return nil
}

func (conf *Configuration) validateBlogs() error {
	set := &dt.Set[string]{}
	ec := &erc.Collector{}
	for idx := range conf.Blog {
		bc := conf.Blog[idx]
		if bc.Name == "" && bc.RepoName == "" {
			ec.Add(fmt.Errorf("blog at index %x is missing a name and a repo name", idx))
		}
		bc.Name = ft.Default(bc.Name, bc.RepoName)
		bc.RepoName = ft.Default(bc.RepoName, bc.Name)

		if set.Check(bc.Name) {
			ec.Add(fmt.Errorf("blog named %s has a duplicate blog configured", bc.Name))
		}

		if set.Check(bc.RepoName) {
			ec.Add(fmt.Errorf("blog with repo %s (named %s) has a duplicate name", bc.RepoName, bc.Name))
		}

		if repos := conf.GetTaggedRepos(bc.RepoName); repos.Len() != 1 {
			ec.Add(fmt.Errorf("blog named %s does not have a corresponding configured repo %s", bc.Name, bc.RepoName))
		}
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
