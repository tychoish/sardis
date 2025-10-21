package srv

import (
	"net/http"

	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
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

func (conf *Configuration) Validate() error {
	conf.DMenuFlags = godmenu.Flags{
		Path:            ft.Default(conf.DMenuFlags.Path, godmenu.DefaultDMenuPath),
		BackgroundColor: ft.Default(conf.DMenuFlags.BackgroundColor, godmenu.DefaultBackgroundColor),
		TextColor:       ft.Default(conf.DMenuFlags.TextColor, godmenu.DefaultTextColor),
		Font:            ft.Default(conf.DMenuFlags.Font, "Source Code Pro-14"),
		Lines:           ft.Default(conf.DMenuFlags.Lines, 16),
		Prompt:          ft.Default(conf.DMenuFlags.Prompt, "=>>"),
	}

	ec := &erc.Collector{}
	ec.Push(conf.Notify.Validate())
	ec.Push(conf.Credentials.Validate())

	// TODO: actually have a client pool
	conf.Telegram.Client = http.DefaultClient

	// TODO fix: there's an IsZero method
	// which checks if the client is set,
	// but users shouldn't have to fix this.

	if ft.Not(conf.Telegram.IsZero()) {
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
