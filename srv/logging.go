package srv

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/coreos/go-systemd/journal"
	"github.com/nwidger/jsoncolor"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/desktop"
	"github.com/tychoish/grip/x/jira"
	"github.com/tychoish/grip/x/system"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/grip/x/twitter"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/sardis/global"
	"github.com/tychoish/sardis/util"
)

type LoggingSettings struct {
	DisableStandardOutput     bool           `bson:"disable_standard_output" json:"disable_standard_output" yaml:"disable_standard_output"`
	EnableJSONFormating       bool           `bson:"enable_json_formatting" json:"enable_json_formatting" yaml:"enable_json_formatting"`
	EnableJSONColorFormatting bool           `bson:"enable_json_color_formatting" json:"enable_json_color_formatting" yaml:"enable_json_color_formatting"`
	DisableSyslog             bool           `bson:"disable_syslog" json:"disable_syslog" yaml:"disable_syslog"`
	Priority                  level.Priority `bson:"priority" json:"priority" yaml:"priority"`
}

type NotifySettings struct {
	Name     string `bson:"name" json:"name" yaml:"name"`
	Target   string `bson:"target" json:"target" yaml:"target"`
	Host     string `bson:"host" json:"host" yaml:"host"`
	User     string `bson:"user" json:"user" yaml:"user"`
	Password string `bson:"password" json:"password" yaml:"password"`
	Disabled bool   `bson:"disabled" json:"disabled" yaml:"disabled"`
}

func (conf *NotifySettings) Validate() error {
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

func WithAppLogger(ctx context.Context, conf LoggingSettings) context.Context {
	sender := SetupLogging(conf)
	grip.SetSender(sender)
	return grip.WithLogger(ctx, grip.NewLogger(sender))
}

func SetupLogging(conf LoggingSettings) send.Sender {
	var sender send.Sender

	if conf.EnableJSONFormating || conf.EnableJSONColorFormatting {
		sender = send.MakePlain()
	} else {
		sender = send.MakeStdError()
	}

	if runtime.GOOS == "linux" && !conf.DisableSyslog && journal.Enabled() {
		syslog := system.MakeDefault()
		syslog.SetName(filepath.Base(os.Args[0]))

		if conf.DisableStandardOutput {
			sender = syslog
		} else {
			sender = send.MakeMulti(syslog, sender)
		}
	}

	switch {
	case conf.EnableJSONColorFormatting:
		sender.SetFormatter(func(m message.Composer) (string, error) {
			out, err := jsoncolor.Marshal(m.Raw())
			if err != nil {
				return "", err
			}
			return string(out), nil
		})
	case conf.EnableJSONFormating:
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	sender.SetPriority(conf.Priority)
	sender.SetName(filepath.Base(os.Args[0]))

	return sender
}

func Twitter(ctx context.Context) grip.Logger {
	return grip.ContextLogger(ctx, global.ContextTwitterLogger)
}

func WithTwitterLogger(ctx context.Context, conf *Credentials) context.Context {
	return grip.WithNewContextLogger(ctx, global.ContextTwitterLogger, func() send.Sender {
		twitter, err := twitter.MakeSender(ctx, &twitter.Options{
			Name:           fmt.Sprint("@", conf.Twitter.Username, "/sardis"),
			ConsumerKey:    conf.Twitter.ConsumerKey,
			ConsumerSecret: conf.Twitter.ConsumerSecret,
			AccessSecret:   conf.Twitter.OauthSecret,
			AccessToken:    conf.Twitter.OauthToken,
		})
		if err != nil {
			err = ers.Wrap(err, "problem constructing twitter sender")
			if srv.HasCleanup(ctx) {
				srv.AddCleanup(ctx, func(context.Context) error { return err })
			} else {
				srv.AddCleanupError(ctx, err)
			}
		}

		twitter.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))
		return twitter
	})
}

func DesktopNotify(ctx context.Context) grip.Logger {
	return grip.ContextLogger(ctx, global.ContextDesktopLogger)
}

func WithDesktopNotify(ctx context.Context) context.Context {
	root := grip.Sender()
	s := desktop.MakeSender()
	s.SetName(global.ApplicationName)
	s.SetPriority(root.Priority())
	sender := send.MakeMulti(s, root)
	return grip.WithContextLogger(ctx, global.ContextDesktopLogger, grip.NewLogger(sender))
}

func RemoteNotify(ctx context.Context) grip.Logger {
	return grip.ContextLogger(ctx, global.ContextRemoteLogger)
}

func WithRemoteNotify(ctx context.Context, conf *Configuration) (out context.Context) {
	var loggers []send.Sender
	root := grip.Sender()
	defer func() {
		out = grip.WithContextLogger(ctx, global.ContextRemoteLogger,
			grip.NewLogger(send.NewMulti(
				root.Name(),
				append(loggers, root),
			)))
	}()

	host := util.GetHostname()

	if conf.Notify.Target != "" && !conf.Notify.Disabled {
		sender, err := xmpp.NewSender(conf.Notify.Target,
			xmpp.ConnectionInfo{
				Hostname:             conf.Notify.Host,
				Username:             conf.Notify.User,
				Password:             conf.Notify.Password,
				DisableTLS:           true,
				AllowUnencryptedAuth: true,
			})
		if err != nil {
			srv.AddCleanupError(ctx, ers.Wrap(err, "setting up notify send issue logger"))
			return
		}

		sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
		sender.SetPriority(root.Priority())

		sender.SetFormatter(func(m message.Composer) (string, error) {
			return fmt.Sprintf("[%s] %s", host, m.String()), nil
		})

		loggers = append(loggers, sender)

		srv.AddCleanup(ctx, func(ctx context.Context) error {
			catcher := &erc.Collector{}
			catcher.Add(sender.Flush(ctx))
			catcher.Add(sender.Close())
			return ers.Wrapf(catcher.Resolve(), "xmpp [%s]", conf.Notify.Name)
		})
	}

	if conf.Telegram.Target != "" {
		opts := conf.Telegram
		opts.BaseURL = "https://api.telegram.org"
		opts.Client = http.DefaultClient
		sender := send.MakeBuffered(telegram.New(opts), time.Second, 10)
		sender.SetPriority(root.Priority())
		sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
		sender.SetFormatter(func(m message.Composer) (string, error) {
			return fmt.Sprintf("[%s] %s", host, m.String()), nil
		})

		loggers = append(loggers, sender)

		srv.AddCleanup(ctx, func(ctx context.Context) error {
			catcher := &erc.Collector{}
			catcher.Add(sender.Flush(ctx))
			catcher.Add(sender.Close())
			return ers.Wrapf(catcher.Resolve(), "telegram [%s]", conf.Telegram.Name)
		})

	}
	if len(loggers) == 0 {
		desktop := desktop.MakeSender()
		desktop.SetName(global.ApplicationName)
		desktop.SetPriority(grip.Sender().Priority())
		loggers = append(loggers, desktop)
	}

	return
}

func JiraIssue(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "jira-issue") }
func WithJiraIssue(ctx context.Context, conf *Configuration) (out context.Context) {
	out = ctx
	root := grip.Sender()
	loggers := []send.Sender{}
	defer func() {
		loggers = append(loggers, root)
		out = grip.WithContextLogger(ctx, "jira-issue", grip.NewLogger(send.MakeMulti(loggers...)))
	}()

	if conf.Credentials.Jira.URL == "" {
		grip.Warning("jira credentials are not configured")
		return
	}

	sender, err := jira.MakeIssueSender(ctx, &jira.Options{
		Name:    conf.Notify.Name,
		BaseURL: conf.Credentials.Jira.URL,
		BasicAuthOpts: jira.BasicAuth{
			UseBasicAuth: true,
			Username:     conf.Credentials.Jira.Username,
			Password:     conf.Credentials.Jira.Password,
		},
	})
	if err != nil {
		srv.AddCleanupError(ctx, ers.Wrap(err, "setting up jira issue logger"))
		return
	}

	sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
	srv.AddCleanup(ctx, func(context.Context) error { return sender.Close() })
	loggers = append(loggers, sender)

	return
}
