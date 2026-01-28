package srv

import (
	"net/http"

	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/godmenu"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/sardis/util"
)

type Configuration struct {
	Logging     LoggingSettings  `bson:"logging" json:"logging" yaml:"logging"`
	Credentials Credentials      `bson:"credentials" json:"credentials" yaml:"credentials"`
	Notify      NotifySettings   `bson:"notify" json:"notify" yaml:"notify"`
	Telegram    telegram.Options `bson:"telegram" json:"telegram" yaml:"telegram"`
	Network     Network          `bson:"network" json:"network" yaml:"network"`
	ConfigPaths []string         `bson:"config_files" json:"config_files" yaml:"config_files"`
	DMenuFlags  godmenu.Flags    `bson:"dmenu" json:"dmenu" yaml:"dmenu"`
	Runtime     struct {
		WithAnnotations     bool   `bson:"annotate" json:"annotate" yaml:"annotate"`
		AnnotationSeparator string `bson:"annotation_separator" json:"annotation_separator" yaml:"annotation_separator"`
	} `bson:"runtime" json:"runtime" yaml:"runtime"`
	ShellHistory struct {
		Paths           []string `bson:"paths" json:"paths" yaml:"paths"`
		ExcludePrefixes []string `bson:"exclude_prefixes" json:"exclude_prefixes" yaml:"exclude_prefixes"`
	} `bson:"shell_history" json:"shell_history" yaml:"shell_history"`
}

func (conf *Configuration) Join(mc *Configuration) {
	if mc == nil {
		return
	}
	conf.ConfigPaths = append(conf.ConfigPaths, mc.ConfigPaths...)
	conf.Notify.Join(&mc.Notify)
	conf.Credentials.Join(&mc.Credentials)
	conf.Logging.Join(&mc.Logging)

	conf.DMenuFlags.BackgroundColor = util.Default(mc.DMenuFlags.BackgroundColor, conf.DMenuFlags.BackgroundColor)
	conf.DMenuFlags.Font = util.Default(mc.DMenuFlags.Font, conf.DMenuFlags.Font)
	conf.DMenuFlags.Lines = util.Default(mc.DMenuFlags.Lines, conf.DMenuFlags.Lines)
	conf.DMenuFlags.Path = util.Default(mc.DMenuFlags.Path, conf.DMenuFlags.Path)
	conf.DMenuFlags.TextColor = util.Default(mc.DMenuFlags.TextColor, conf.DMenuFlags.TextColor)
	conf.DMenuFlags.Prompt = util.Default(mc.DMenuFlags.Prompt, conf.DMenuFlags.Prompt)
	conf.DMenuFlags.Monitor = util.Default(mc.DMenuFlags.Monitor, conf.DMenuFlags.Monitor)
	conf.DMenuFlags.WindowID = util.Default(mc.DMenuFlags.WindowID, conf.DMenuFlags.WindowID)

	conf.Telegram.Name = util.Default(mc.Telegram.Name, conf.Telegram.Name)
	conf.Telegram.Target = util.Default(mc.Telegram.Target, conf.Telegram.Target)
	conf.Telegram.Token = util.Default(mc.Telegram.Token, conf.Telegram.Token)
	conf.Telegram.Client = util.Default(mc.Telegram.Client, conf.Telegram.Client)

	conf.ShellHistory.Paths = irt.Collect(irt.RemoveZeros(
		irt.Keep(irt.Convert(
			irt.Slice(conf.ShellHistory.Paths),
			util.TryExpandHomeDir,
		), util.FileExists),
	))
}

type Credentials struct {
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
	AWS []struct {
		Profile string `bson:"profile" json:"profile" yaml:"profile"`
		Key     string `bson:"key" json:"key" yaml:"key"`
		Secret  string `bson:"secret" json:"secret" yaml:"secret"`
		Token   string `bson:"token" json:"token" yaml:"token"`
	} `bson:"aws" json:"aws" yaml:"aws"`
}

func (conf *Credentials) Join(mc *Credentials) {
	if mc == nil {
		return
	}
	conf.Path = util.Default(mc.Path, conf.Path)

	conf.Twitter.Username = util.Default(mc.Twitter.Username, conf.Twitter.Username)
	conf.Twitter.ConsumerKey = util.Default(mc.Twitter.ConsumerKey, conf.Twitter.ConsumerKey)
	conf.Twitter.ConsumerSecret = util.Default(mc.Twitter.ConsumerSecret, conf.Twitter.ConsumerSecret)
	conf.Twitter.OauthToken = util.Default(mc.Twitter.OauthToken, conf.Twitter.OauthToken)
	conf.Twitter.OauthSecret = util.Default(mc.Twitter.OauthSecret, conf.Twitter.OauthSecret)

	conf.Jira.Username = util.Default(mc.Jira.Username, conf.Jira.Username)
	conf.Jira.Password = util.Default(mc.Jira.Password, conf.Jira.Password)
	conf.Jira.URL = util.Default(mc.Jira.URL, conf.Jira.URL)

	conf.GitHub.Username = util.Default(mc.GitHub.Username, conf.GitHub.Username)
	conf.GitHub.Password = util.Default(mc.GitHub.Password, conf.GitHub.Password)
	conf.GitHub.Token = util.Default(mc.GitHub.Token, conf.GitHub.Token)

	conf.AWS = append(conf.AWS, mc.AWS...)
}

func (conf *Configuration) Validate() error {
	conf.DMenuFlags = godmenu.Flags{
		Path:            util.Default(conf.DMenuFlags.Path, godmenu.DefaultDMenuPath),
		BackgroundColor: util.Default(conf.DMenuFlags.BackgroundColor, godmenu.DefaultBackgroundColor),
		TextColor:       util.Default(conf.DMenuFlags.TextColor, godmenu.DefaultTextColor),
		Font:            util.Default(conf.DMenuFlags.Font, "Source Code Pro-14"),
		Lines:           util.Default(conf.DMenuFlags.Lines, 16),
		Prompt:          util.Default(conf.DMenuFlags.Prompt, "=>>"),
	}

	ec := &erc.Collector{}
	ec.Push(conf.Notify.Validate())
	ec.Push(conf.Credentials.Validate())

	// TODO: actually have a client pool
	conf.Telegram.Client = http.DefaultClient

	// TODO fix: there's an IsZero method
	// which checks if the client is set,
	// but users shouldn't have to fix this.

	if !conf.Telegram.IsZero() {
		ec.Push(conf.Telegram.Validate())
	}

	for idx := range conf.ConfigPaths {
		conf.ConfigPaths[idx] = util.TryExpandHomeDir(conf.ConfigPaths[idx])
	}

	return ec.Resolve()
}

func (conf *Configuration) DMenu() godmenu.Arg { return godmenu.WithFlags(&conf.DMenuFlags) }

func (conf *Credentials) Validate() error {
	if conf.Path == "" {
		return nil
	}

	var err error
	conf.Path, err = homedir.Expand(conf.Path)
	if err != nil {
		return err
	}

	return util.UnmarshalFile(conf.Path, &conf)
}
