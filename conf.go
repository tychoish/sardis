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
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis/repo"
	sutil "github.com/tychoish/sardis/util"
)

type Configuration struct {
	Settings         Settings             `bson:"settings" json:"settings" yaml:"settings"`
	Repo             []repo.Configuration `bson:"repo" json:"repo" yaml:"repo"`
	Links            []LinkConf           `bson:"links" json:"links" yaml:"links"`
	Hosts            []HostConf           `bson:"hosts" json:"hosts" yaml:"hosts"`
	System           SystemConf           `bson:"system" json:"system" yaml:"system"`
	Commands         []CommandGroupConf   `bson:"commands" json:"commands" yaml:"commands"`
	TerminalCommands []CommandConf        `bson:"terminals" json:"terminals" yaml:"terminals"`
	Blog             []BlogConf           `bson:"blog" json:"blog" yaml:"blog"`
	Menus            []MenuConf           `bson:"menu" json:"menu" yaml:"menu"`

	generatedLocalOps bool
	repoTagsEvaluated bool
	linkedFilesRead   bool

	repoTags map[string][]*repo.Configuration
	caches   struct {
		commandGroups    adt.Once[map[string]CommandGroupConf]
		allCommdands     adt.Once[[]CommandConf]
		comandGroupNames adt.Once[[]string]
		validation       adt.Once[error]
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

type CommandGroupConf struct {
	Name          string                 `bson:"name" json:"name" yaml:"name"`
	Aliases       []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory     string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment   dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	CmdNamePrefix string                 `bson:"command_name_prefix" json:"command_name_prefix" yaml:"command_name_prefix"`
	Command       string                 `bson:"default_command" json:"default_command" yaml:"default_command"`
	Notify        *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background    *bool                  `bson:"background" json:"background" yaml:"background"`
	Host          *string                `bson:"host" json:"host" yaml:"host"`
	Commands      []CommandConf          `bson:"commands" json:"commands" yaml:"commands"`

	unaliasedName string
	exportCache   *adt.Once[map[string]CommandConf]
}

func (cg *CommandGroupConf) NamesAtIndex(idx int) []string {
	fun.Invariant.Ok(idx >= 0 && idx < len(cg.Commands), "command out of bounds", cg.Name)
	ops := []string{}
	var base string
	if cg.CmdNamePrefix == "" {
		base = "."
	} else {
		base = fmt.Sprint(".", cg.CmdNamePrefix, ".")
	}

	for _, grp := range append([]string{cg.Name}, cg.Aliases...) {
		cmd := cg.Commands[idx]
		for _, name := range append([]string{cmd.Name}, cmd.Aliases...) {
			ops = append(ops, fmt.Sprint(grp, base, name))
		}
	}

	return ops
}

type CommandConf struct {
	Name            string                 `bson:"name" json:"name" yaml:"name"`
	GroupName       string                 `bson:"-" json:"-" yaml:"-"`
	Aliases         []string               `bson:"aliases" json:"aliases" yaml:"aliases"`
	Directory       string                 `bson:"directory" json:"directory" yaml:"directory"`
	Environment     dt.Map[string, string] `bson:"env" json:"env" yaml:"env"`
	Command         string                 `bson:"command" json:"command" yaml:"command"`
	Commands        []string               `bson:"commands" json:"commands" yaml:"commands"`
	OverrideDefault bool                   `bson:"override_default" json:"override_default" yaml:"override_default"`
	Notify          *bool                  `bson:"notify" json:"notify" yaml:"notify"`
	Background      *bool                  `bson:"bson" json:"bson" yaml:"bson"`
	Host            *string                `bson:"host" json:"host" yaml:"host"`

	// if possible call the operation rather
	// than execing
	operation     fun.Worker
	unaliasedName string
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

	ec.Add(conf.Settings.Validate())
	ec.Add(conf.System.Arch.Validate())
	conf.expandLinkedFiles(ec)

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
		conf.Links[idx].Target = strings.ReplaceAll(conf.Links[idx].Target, "{{hostname}}", conf.Settings.Runtime.Hostname)
	}

	for idx := range conf.Hosts {
		ec.Wrapf(conf.Hosts[idx].Validate(), "%d of %T is not valid", idx, conf.Hosts[idx])
	}

	conf.expandLocalNativeOps()
	for idx := range conf.Commands {
		ec.Wrapf(conf.Commands[idx].Validate(), "%d of %T is not valid", idx, conf.Commands[idx])
	}
	conf.resolveCommands()

	conf.mapReposByTags()
	ec.Wrap(conf.validateBlogs(), "blog build/push configuration is invalid")

	return ec.Resolve()
}

func (conf *Configuration) expandLocalNativeOps() {
	if conf.generatedLocalOps {
		return
	}
	defer func() { conf.generatedLocalOps = true }()

	for idx := range conf.Repo {
		repo := conf.Repo[idx]
		if repo.Disabled {
			continue
		}
		if !repo.Fetch && !repo.LocalSync {
			continue
}

		cg := CommandGroupConf{
			Name:          "repo",
			Notify:        ft.Ptr(repo.Notify),
			CmdNamePrefix: repo.Name,
			Commands: []CommandConf{{
				Name:            "status",
				Directory:       repo.Path,
				OverrideDefault: true,
				Command:         "alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command 'sardis repo status {{prefix}}'",
			}},
		}

		if repo.Fetch || repo.LocalSync {
			cg.Commands = append(cg.Commands, CommandConf{
				Name:      "fetch",
				operation: repo.FetchJob(),
			})
		}

		if repo.LocalSync {
			cg.Commands = append(cg.Commands, CommandConf{
				Name:      "update",
				operation: repo.FullSync(),
			})
		}

		conf.Commands = append(conf.Commands, cg)
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

		// TODO these have a worker function already
		// implemented in units for setup.
		conf.Commands = append(conf.Commands, CommandGroupConf{
			Name:          "systemd.service",
			Directory:     conf.Settings.Runtime.Hostname,
			Notify:        ft.Ptr(true),
			CmdNamePrefix: service.Name,
			Command:       fmt.Sprintf("%s {{name}} %s", command, service.Unit),
			Commands: []CommandConf{
				{Name: "restart"},
				{Name: "stop"},
				{Name: "start"},
				{Name: "enable"},
				{Name: "disable"},
				{Name: "setup", Command: defaultState},
				{
					Name:            "logs",
					Command:         fmt.Sprintf("alacritty msg create-window --title {{group.name}}.{{prefix}}.{{name}} --command journalctl --follow --pager-end --catalog --unit %s", service.Unit),
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
}

func (conf *Configuration) expandLinkedFiles(ec *erc.Collector) {
	if conf.linkedFilesRead {
		return
	}
	defer func() { conf.linkedFilesRead = true }()

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
				ec.Add(fmt.Errorf("problem reading linked config file %q: %w", fn, err))
				return
			}
			if conf == nil {
				ec.Add(fmt.Errorf("nil configuration for %q", fn))
				return
			}

			ec.Whenf(len(conf.Settings.ConfigPaths) != 0,
				"nested file %q specified additional files %v, which is invalid",
				fn, conf.Settings.ConfigPaths)
			pipe <- conf
		}(idx, fileName)
	}

	wg.Wait()
	close(pipe)

	confs := make([]*Configuration, 0, len(conf.Settings.ConfigPaths))
	for c := range pipe {
		confs = append(confs, c)
	}

	conf.Merge(confs...)
}

func (conf *Configuration) resolveCommands() {
	// expand aliases
	if len(conf.Commands) == 0 {
		return
	}
	withAliases := make([]CommandGroupConf, 0, len(conf.Commands)+len(conf.Commands)/2+1)
	for idx := range conf.Commands {
		cg := conf.Commands[idx]
		withAliases = append(withAliases, cg)
		if len(cg.Aliases) == 0 {
			continue
		}

		for _, alias := range cg.Aliases {
			acg := cg
			acg.Aliases = nil
			acg.Name = alias
			withAliases = append(withAliases, acg)
		}
		cg.Aliases = nil
	}
	conf.Commands = withAliases

	index := make(map[string]int, len(conf.Commands))
	haveMerged := false
	for idx := range conf.Commands {
		lhn := conf.Commands[idx].Name

		if _, ok := index[lhn]; !ok {
			index[lhn] = idx
			continue
		}

		cg := &conf.Commands[index[lhn]]
		cg.Merge(conf.Commands[idx])
		conf.Commands[index[lhn]] = *cg
		haveMerged = true
	}

	if !haveMerged {
		return
	}

	// get map of names -> indexes as an ordered sequence
	sparse := dt.NewMap(index).Pairs()

	// reorder it because it came off of a default map in random order
	sparse.SortQuick(cmp.LessThanConverter(func(p dt.Pair[string, int]) int { return p.Value }))

	// make an output structure
	resolved := dt.NewSlice(make([]CommandGroupConf, 0, len(index)))

	// read the resolution inside out...
	//
	// ⬇️ ingest the contents of the converted and reordered stream
	// into the resolved slice
	resolved.Populate(
		// use the .Index accessor to pull the groups out of the
		// stream of sparse indexes of now merged groups ⬇️
		fun.MakeConverter(dt.NewSlice(conf.Commands).Index).Stream(
			// ⬇️ convert it into the (sparse) indexes of merged groups ⬆
			fun.MakeConverter(func(p dt.Pair[string, int]) int { return p.Value }).Stream(
				// ⬇️ take the (now ordered) pairs that we merged and ⬆
				sparse.Stream(),
			),
		),
	).Must().Wait()

	conf.Commands = resolved
}

func (conf *Configuration) Merge(mcfs ...*Configuration) {
	mcfs = append([]*Configuration{}, mcfs...)
	reposAdded := 0

	for idx := range mcfs {
		mcf := mcfs[idx]
		if mcf == nil {
			continue
		}

		reposAdded += len(mcf.Repo)
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

	if reposAdded > 0 {
		conf.repoTagsEvaluated = false
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

func (conf *Configuration) mapReposByTags() {
	defer func() { conf.repoTagsEvaluated = true }()
	if conf.repoTagsEvaluated {
		return
	}

	conf.repoTags = make(map[string][]*repo.Configuration)

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

func (conf *CommandGroupConf) Validate() error {
	var err error
	home := util.GetHomedir()
	ec := &erc.Collector{}

	ec.When(conf.Name == "", ers.Error("command group must have name"))
	if conf.unaliasedName == "" {
		conf.unaliasedName = conf.Name
	}

	var aliased []CommandConf

	for idx := range conf.Commands {
		cmd := conf.Commands[idx]
		cmd.GroupName = conf.Name

		ec.Whenf(cmd.Name == "", "command in group [%s](%d) must have a name", conf.Name, idx)

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

		if cmd.Command == "" {
			ec.Whenf(cmd.OverrideDefault, "cannot override default without an override, in group [%s] command [%s](%d)", conf.Name, cmd.Name, idx)
			if conf.Command != "" {
				cmd.Command = conf.Command
			} else {
				cmd.Command = cmd.Name
			}
			ec.Whenf(cmd.Command == "", "cannot resolve default command in group [%s] command [%s](%d)", conf.Name, cmd.Name, idx)
		}

		ec.Whenf(strings.Contains(cmd.Command, " {{command}}"), "unresolveable token found in group [%s] command [%s](%d)", conf.Name, cmd.Name, idx)

		if conf.Command != "" && !cmd.OverrideDefault {
			cmd.Command = strings.ReplaceAll(conf.Command, "{{command}}", cmd.Command)
		}

		cmd.Command = strings.ReplaceAll(cmd.Command, "{{name}}", cmd.Name)
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{group.name}}", conf.Name)
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{host}}", ft.Ref(conf.Host))
		cmd.Command = strings.ReplaceAll(cmd.Command, "{{prefix}}", conf.CmdNamePrefix)

		if len(cmd.Aliases) >= 1 {
			cmd.Command = strings.ReplaceAll(cmd.Command, "{{alias}}", cmd.Aliases[0])
		}
		for idx, alias := range cmd.Aliases {
			cmd.Command = strings.ReplaceAll(cmd.Command, fmt.Sprintf("{{alias[%d]}}", idx), alias)
		}

		for idx := range cmd.Commands {
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{command}}", cmd.Command)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{name}}", cmd.Name)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{host}}", ft.Ref(conf.Host))
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{group.name}}", conf.Name)
			cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{prefix}}", conf.Name)

			if len(cmd.Aliases) >= 1 {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], "{{alias}}", cmd.Aliases[0])
			}

			for idx, alias := range cmd.Aliases {
				cmd.Commands[idx] = strings.ReplaceAll(cmd.Commands[idx], fmt.Sprintf("{{alias[%d]}}", idx), alias)
			}
		}

		if conf.CmdNamePrefix != "" {
			cmd.Name = fmt.Sprintf("%s.%s", conf.CmdNamePrefix, cmd.Name)
		}

		cmd.Notify = ft.Default(cmd.Notify, conf.Notify)
		cmd.Background = ft.Default(cmd.Background, conf.Background)
		cmd.Host = ft.Default(cmd.Host, conf.Host)
		cmd.Directory, err = homedir.Expand(cmd.Directory)
		ec.Add(ers.Wrapf(err, "command group(%s)  %q at %d", cmd.GroupName, cmd.Name, idx))

		for _, alias := range cmd.Aliases {
			acmd := cmd
			if conf.CmdNamePrefix != "" {
				acmd.Name = fmt.Sprintf("%s.%s", conf.CmdNamePrefix, alias)
			} else {
				acmd.Name = alias
			}
			acmd.Aliases = nil
			acmd.unaliasedName = cmd.Name
			aliased = append(aliased, acmd)
		}
		cmd.Aliases = nil
		conf.Commands[idx] = cmd
	}
	conf.Commands = append(conf.Commands, aliased...)

	return ec.Resolve()
}

func (conf *CommandConf) NamePrime() string { return ft.Default(conf.unaliasedName, conf.Name) }

func (conf *CommandGroupConf) Merge(rhv CommandGroupConf) bool {
	if conf.Name != rhv.Name {
		return false
	}

	conf.Commands = append(conf.Commands, rhv.Commands...)
	conf.Command = ""
	conf.Aliases = nil
	conf.Environment = nil
	return true
}

func (conf *Configuration) ExportAllCommands() []CommandConf {
	return conf.caches.allCommdands.Call(conf.doExportAllCommands)
}
func (conf *Configuration) doExportAllCommands() []CommandConf {
	out := dt.NewSlice([]CommandConf{})

	for _, cg := range conf.Commands {
		if hn, ok := ft.RefOk(cg.Host); ok && hn == conf.Settings.Runtime.Hostname && !conf.Settings.Runtime.IncludeLocalSHH {
			continue
		}

		for cidx := range cg.Commands {
			if hn, ok := ft.RefOk(cg.Commands[cidx].Host); ok && hn == conf.Settings.Runtime.Hostname && !conf.Settings.Runtime.IncludeLocalSHH {
				continue
			}

			cmd := cg.Commands[cidx]
			cmd.Name = fmt.Sprintf("%s.%s", cg.Name, cmd.Name)
			out = append(out, cmd)
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

			out = append(out, cmd)
		}
	}

	return out
}

func (conf *Configuration) ExportCommandGroups() dt.Map[string, CommandGroupConf] {
	return conf.caches.commandGroups.Call(conf.doExportCommandGroups)
}
func (conf *Configuration) ExportGroupNames() dt.Slice[string] {
	return conf.caches.comandGroupNames.Call(conf.doExportGroupNames)
}
func (conf *Configuration) doExportGroupNames() []string {
	return fun.NewGenerator(conf.ExportCommandGroups().Keys().Slice).Force().Resolve()
}

func (conf *Configuration) doExportCommandGroups() map[string]CommandGroupConf {
	out := make(map[string]CommandGroupConf, len(conf.Commands))
	for idx := range conf.Commands {
		group := conf.Commands[idx]
		out[group.Name] = group
		for idx := range group.Aliases {
			alias := group.Aliases[idx]
			ag := conf.Commands[idx]
			out[alias] = ag
		}
	}
	return out
}

func (conf *CommandConf) Worker() fun.Worker {
	if conf.operation != nil {
		return conf.operation
	}

	sender := grip.Sender()
	hn := util.GetHostname()

	return func(ctx context.Context) error {
		return jasper.Context(ctx).CreateCommand(ctx).
			ID(fmt.Sprintf("CMD(%s).HOST(%s).NUM(%d)", conf.Name, util.GetHostname(), 1+len(conf.Commands))).
			Directory(conf.Directory).
			Environment(conf.Environment).
			AddEnv(EnvVarSardisLogQuietStdOut, "true").
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Error, sender).
			Background(ft.Ref(conf.Background)).
			Append(conf.Command).
			Append(conf.Commands...).
			Prerequisite(func() bool {
				grip.Info(message.BuildPair().
					Pair("op", conf.Name).
					Pair("host", hn).
					Pair("dir", conf.Directory).
					Pair("cmd", conf.Command).
					Pair("cmds", conf.Commands))
				return true
			}).
			PostHook(func(err error) error {
				if err != nil {
					m := message.WrapError(err, conf.Name)
					DesktopNotify(ctx).Error(m)
					grip.Critical(err)
					return err
				}
				DesktopNotify(ctx).Notice(message.Whenln(ft.Ref(conf.Notify), conf.Name, "completed"))
				return nil
			}).Run(ctx)
	}

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
