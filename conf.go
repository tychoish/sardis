package sardis

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/mitchellh/go-homedir"

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
	sutil "github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings         Settings           `bson:"settings" json:"settings" yaml:"settings"`
	Repo             []RepoConf         `bson:"repo" json:"repo" yaml:"repo"`
	Links            []LinkConf         `bson:"links" json:"links" yaml:"links"`
	Hosts            []HostConf         `bson:"hosts" json:"hosts" yaml:"hosts"`
	System           SystemConf         `bson:"system" json:"system" yaml:"system"`
	Commands         []CommandGroupConf `bson:"commands" json:"commands" yaml:"commands"`
	TerminalCommands []CommandConf      `bson:"terminals" json:"terminals" yaml:"terminals"`
	Blog             []BlogConf         `bson:"blog" json:"blog" yaml:"blog"`
	Menus            []MenuConf         `bson:"menu" json:"menu" yaml:"menu"`

	repoTags         map[string][]*RepoConf
	indexedRepoCount int
	linkedFilesRead  bool
}

type RepoConf struct {
	Name       string   `bson:"name" json:"name" yaml:"name"`
	Path       string   `bson:"path" json:"path" yaml:"path"`
	Remote     string   `bson:"remote" json:"remote" yaml:"remote"`
	RemoteName string   `bson:"remote_name" json:"remote_name" yaml:"remote_name"`
	Branch     string   `bson:"branch" json:"branch" yaml:"branch"`
	LocalSync  bool     `bson:"sync" json:"sync" yaml:"sync"`
	Fetch      bool     `bson:"fetch" json:"fetch" yaml:"fetch"`
	Notify     bool     `bson:"notify" json:"notify" yaml:"notify"`
	Disabled   bool     `bson:"disabled" json:"disabled" yaml:"disabled"`
	Pre        []string `bson:"pre" json:"pre" yaml:"pre"`
	Post       []string `bson:"post" json:"post" yaml:"post"`
	Mirrors    []string `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
	Tags       []string `bson:"tags" json:"tags" yaml:"tags"`
	Deploy     []string `bson:"deploy" json:"deploy" yaml:"deploy"`
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
	Hostname            string           `bson:"-" json:"-" yaml:"-"`
	Logging             LoggingConf      `bson:"logging" json:"logging" yaml:"logging"`
	ConfigPaths         []string         `bson:"config_files" json:"config_files" yaml:"config_files"`
	DMenu               godmenu.Flags    `bson:"dmenu" json:"dmenu" yaml:"dmenu"`
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

type CommandGroupConf struct {
	Name        string                 `bson:"name" json:"name" yaml:"name"`
	Aliases     []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory   string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	Command     string                 `bson:"default_command" json:"default_command" yaml:"default_command"`
	Notify      *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background  *bool                  `bson:"background" json:"background" yaml:"background"`
	Commands    []CommandConf          `bson:"commands" json:"commands" yaml:"commands"`
}

type CommandConf struct {
	Name            string                 `bson:"name" json:"name" yaml:"name"`
	Aliases         []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory       string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment     dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	Command         string                 `bson:"command" json:"command" yaml:"command"`
	Commands        []string               `bson:"commands" json:"commands" yaml:"commands"`
	OverrideDefault bool                   `bson:"override_default" json:"override_default" yaml:"override_default"`
	Notify          *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background      *bool                  `bson:"bson" json:"bson" yaml:"bson"`
}

type BlogConf struct {
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
	out := &Configuration{}

	if err := sutil.UnmarshalFile(fn, out); err != nil {
		return nil, fmt.Errorf("problem unmarshaling config data: %w", err)
	}

	if err := out.Validate(); err != nil {
		return nil, err
	}

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

func (conf *Configuration) Validate() error {
	ec := &erc.Collector{}

	ec.Add(conf.Settings.Validate())
	ec.Add(conf.System.Arch.Validate())

	conf.expandLinkedFiles(ec)

	for idx := range conf.System.Services {
		ec.Wrapf(conf.System.Services[idx].Validate(), "%d of %T is not valid", idx, len(conf.System.Services), conf.System.Services[idx])
	}

	for idx := range conf.Repo {
		ec.Wrapf(conf.Repo[idx].Validate(), "%d/%d of %T is not valid", idx, len(conf.Repo), conf.Repo[idx])
	}

	conf.Links = conf.expandLinks(ec)
	for idx := range conf.Links {
		ec.Wrapf(conf.Links[idx].Validate(), "%d/%d of %T is not valid", idx, len(conf.Links), conf.Links[idx])
		conf.Links[idx].Target = strings.ReplaceAll(conf.Links[idx].Target, "{{hostname}}", conf.Settings.Hostname)
	}

	for idx := range conf.Hosts {
		ec.Wrapf(conf.Hosts[idx].Validate(), "%d of %T is not valid", idx, conf.Hosts[idx])
	}

	for idx := range conf.Commands {
		ec.Wrapf(conf.Commands[idx].Validate(), "%d of %T is not valid", idx, conf.Commands[idx])
	}
	for idx := range conf.Menus {
		ec.Whenf(conf.Menus[idx].Name == "", "must specify name for dmenu spec at item %d", idx)
		ec.Wrapf(conf.Menus[idx].Validate(), "%d of %T is not valid", idx, conf.Menus[idx])
	}

	if conf.shouldIndexRepos() {
		conf.mapReposByTags()
	}

	return ec.Resolve()
}

func (conf *Configuration) expandLinkedFiles(catcher *erc.Collector) {
	if conf.linkedFilesRead {
		return
	}
	defer func() { conf.linkedFilesRead = true }()

	pipe := make(chan *Configuration, len(conf.Settings.ConfigPaths))

	wg := &sync.WaitGroup{}
	for idx, fn := range conf.Settings.ConfigPaths {
		if _, err := os.Stat(fn); os.IsNotExist(err) {
			grip.Warning(message.Fields{
				"file": fn,
				"msg":  "config file does not exist",
			})
			continue
		}

		wg.Add(1)
		go func(idx int, fn string) {
			defer wg.Done()
			conf, err := LoadConfiguration(fn)
			if err != nil {
				catcher.Add(fmt.Errorf("problem reading linked config file %q: %w", fn, err))
				return
			}
			if conf == nil {
				catcher.Add(fmt.Errorf("nil configuration for %q", fn))
				return
			}

			catcher.Whenf(len(conf.Settings.ConfigPaths) != 0,
				"nested file %q specified additional files %v, which is invalid",
				fn, conf.Settings.ConfigPaths)
			pipe <- conf
		}(idx, fn)
	}

	wg.Wait()
	close(pipe)

	confs := make([]*Configuration, 0, len(conf.Settings.ConfigPaths))
	for c := range pipe {
		confs = append(confs, c)
	}

	conf.Merge(confs...)
}

func (conf *Configuration) Merge(mcfs ...*Configuration) {
	for idx := range mcfs {
		mcf := mcfs[idx]
		if mcf == nil {
			continue
		}

		conf.Blog = append(conf.Blog, mcf.Blog...)
		conf.Commands = append(conf.Commands, mcf.Commands...)
		conf.Hosts = append(conf.Hosts, mcf.Hosts...)
		conf.Links = append(conf.Links, mcf.Links...)
		conf.Menus = append(conf.Menus, mcf.Menus...)
		conf.Repo = append(conf.Repo, mcf.Repo...)
		conf.System.Arch.AurPackages = append(conf.System.Arch.AurPackages, mcf.System.Arch.AurPackages...)
		conf.System.Arch.Packages = append(conf.System.Arch.Packages, mcf.System.Arch.Packages...)
		conf.System.GoPackages = append(conf.System.GoPackages, mcf.System.GoPackages...)
		conf.System.Services = append(conf.System.Services, mcf.System.Services...)
		conf.TerminalCommands = append(conf.TerminalCommands, mcf.TerminalCommands...)
	}

	if conf.shouldIndexRepos() {
		conf.mapReposByTags()
	}
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

func (conf *Configuration) GetTaggedRepos(tags ...string) dt.Slice[RepoConf] {
	if len(tags) == 0 {
		return nil
	}

	var out []RepoConf

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

func (conf *Configuration) shouldIndexRepos() bool { return len(conf.Repo) != conf.indexedRepoCount }

func (conf *Configuration) mapReposByTags() {
	defer func() { conf.indexedRepoCount = len(conf.Repo) }()

	conf.repoTags = make(map[string][]*RepoConf)

	for idx := range conf.Repo {
		for _, tag := range conf.Repo[idx].Tags {
			rted := conf.repoTags[tag]
			rted = append(rted, &conf.Repo[idx])
			conf.repoTags[tag] = rted
		}

		name := conf.Repo[idx].Name
		rned, ok := conf.repoTags[name]

		grip.WarningWhen(ok, message.Fields{
			"name":    name,
			"message": "repo name collides with a configured tag",
		})

		rned = append(rned, &conf.Repo[idx])
		conf.repoTags[name] = rned
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
	conf.Hostname = makeErrorHandler[string](catcher.Push)(os.Hostname())

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

func (conf *RepoConf) Validate() error {
	if conf.Branch == "" {
		conf.Branch = "master"
	}

	if conf.RemoteName == "" {
		conf.RemoteName = "origin"
	}

	if conf.Remote == "" {
		return fmt.Errorf("'%s' does not specify a remote", conf.Name)
	}

	if conf.Fetch && conf.LocalSync {
		return errors.New("cannot specify sync and fetch")
	}

	conf.Path = util.TryExpandHomedir(conf.Path)
	conf.Post = sutil.TryExpandHomeDirs(conf.Post)
	conf.Pre = sutil.TryExpandHomeDirs(conf.Pre)

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
	if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
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

func (conf *CommandGroupConf) Validate() error {
	var err error
	home := util.GetHomedir()
	catcher := &erc.Collector{}

	catcher.When(conf.Name == "", ers.Error("command group must have name"))

	for idx := range conf.Commands {
		cmd := conf.Commands[idx]

		catcher.Whenf(cmd.Name == "", "commands [%d] must have a name", idx)

		if cmd.Directory == "" {
			cmd.Directory = home
		}

		if conf.Environment != nil || cmd.Environment != nil {
			env := dt.Map[string, string]{}
			if conf.Environment != nil {
				env.ExtendWithStream(conf.Environment.Stream()).Ignore().Wait()
			}
			if cmd.Environment != nil {
				env.ExtendWithStream(cmd.Environment.Stream()).Ignore().Wait()
			}

			cmd.Environment = env
		}

		if conf.Command != "" {
			if cmd.Command == "" {
				catcher.Whenf(cmd.OverrideDefault, "command %q in group %q is unspecified with OverrideDefault", cmd.Name, conf.Name)
				cmd.Command = conf.Command
			} else if !cmd.OverrideDefault && strings.Contains(conf.Command, "{{command}}") {
				cmd.Command = strings.ReplaceAll(conf.Command, "{{command}}", cmd.Command)
			}

			cmd.Command = strings.ReplaceAll(cmd.Command, "{{name}}", cmd.Name)
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.name}}", conf.Name)

			if len(cmd.Aliases) >= 1 {
				cmd.Command = strings.ReplaceAll(cmd.Command, "{{alias}}", cmd.Aliases[0])
			}
			for idx, alias := range cmd.Aliases {
				cmd.Command = strings.ReplaceAll(cmd.Command, fmt.Sprintf("{{alias[%d]}}", idx), alias)
			}
		}

		for idx := range cmd.Commands {
			if !cmd.OverrideDefault && strings.Contains(conf.Command, "{{command}}") {
				catcher.Whenf(cmd.OverrideDefault, "command %q.%d in group %q is unspecified with OverrideDefault", cmd.Name, idx, conf.Name)
				cmd.Commands[idx] = strings.ReplaceAll(conf.Command, "{{command}}", cmd.Commands[idx])
			}

			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{name}}", cmd.Name)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{group.name}}", conf.Name)
			if len(cmd.Aliases) >= 1 {
				cmd.Command = strings.ReplaceAll(cmd.Command, "{{alias}}", cmd.Aliases[0])
			}
			for idx, alias := range cmd.Aliases {
				cmd.Command = strings.ReplaceAll(cmd.Command, fmt.Sprintf("{{alias[%d]}}", idx), alias)
			}
		}

		cmd.Directory, err = homedir.Expand(cmd.Directory)
		catcher.Add(ers.Wrapf(err, "command %q at %d", cmd.Name, idx))

		conf.Commands[idx] = cmd
	}

	return catcher.Resolve()
}

func (conf *Configuration) ExportAllCommands() map[string]CommandConf {
	out := dt.NewMap(map[string]CommandConf{})
	for _, cg := range conf.Commands {
		groupNames := append([]string{cg.Name}, cg.Aliases...)
		for _, groupName := range groupNames {
			for cidx := range cg.Commands {
				cgName := cg.Commands[cidx].Name
				cmdNames := append(make([]string, 0, 2+len(cg.Commands[cidx].Aliases)),
					fmt.Sprintf("%s.%s", groupName, cgName))

				for _, a := range cg.Commands[cidx].Aliases {
					cmdNames = append(cmdNames, fmt.Sprintf("%s.%s", groupName, a))
				}

				for _, name := range cmdNames {
					cmd := cg.Commands[cidx]

					cmd.Notify = ft.Default(cmd.Notify, cg.Notify)
					cmd.Background = ft.Default(cmd.Background, cg.Background)

					out[name] = cmd
				}
			}
		}
	}
	for _, menus := range conf.Menus {
		for _, operation := range menus.Selections {
			var cmd CommandConf
			cmd.Name = fmt.Sprintf("%s.%s", menus.Name, operation)

			if menus.Command == "" {
				cmd.Command = operation
			} else {
				cmd.Command = fmt.Sprintf("%s %s", menus.Command, operation)
			}

			cmd.Notify = ft.Ptr(menus.Notify)
			cmd.Background = ft.Ptr(menus.Background)

			out[cmd.Name] = cmd
		}
	}

	return out
}

func (conf *Configuration) ExportCommandGroups() map[string]CommandGroupConf {
	out := make(map[string]CommandGroupConf, len(conf.Commands))
	for idx := range conf.Commands {
		group := conf.Commands[idx]
		out[group.Name] = group
		for idx := range group.Aliases {
			out[group.Aliases[idx]] = group
		}
	}
	return out
}

func (conf *CommandGroupConf) ExportCommands() map[string]CommandConf {
	out := make(map[string]CommandConf, len(conf.Commands))
	for idx := range conf.Commands {
		cmd := conf.Commands[idx]
		out[cmd.Name] = cmd
		for idx := range cmd.Aliases {
			out[cmd.Aliases[idx]] = cmd
		}
	}
	return out
}

func (conf *Configuration) AlacrittySocket() string {
	if conf.Settings.AlacrittySocketPath == "" {
		conf.Settings.AlacrittySocketPath = ft.Must(sutil.GetAlacrittySocketPath())
	}
	return conf.Settings.AlacrittySocketPath
}

func (conf *Configuration) SSHAgentSocket() string {
	if conf.Settings.SSHAgentSocketPath == "" {
		conf.Settings.SSHAgentSocketPath = ft.Must(sutil.GetSSHAgentPath())
	}
	return conf.Settings.SSHAgentSocketPath

}
