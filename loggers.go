package sardis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/tychoish/fun/erc"
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
	"github.com/tychoish/sardis/util"
)

func SetupLogging(ctx context.Context, conf *Configuration) error {
	sender := grip.Sender()

	if runtime.GOOS == "linux" {
		syslog, err := system.MakeDefault()
		if err != nil {
			return err
		}

		if conf.Settings.Logging.DisableStandardOutput {
			sender = syslog
		} else {
			sender = send.MakeMulti(syslog, sender)
		}
	}

	if conf.Settings.Logging.EnableJSONFormating {
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	sender.SetName(filepath.Base(os.Args[0]))
	sender.SetPriority(conf.Settings.Logging.Priority)
	grip.SetGlobalLogger(grip.NewLogger(sender))

	return nil
}

func Twitter(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "twitter") }
func WithTwitterLogger(ctx context.Context, conf *Configuration) context.Context {
	return grip.WithNewContextLogger(ctx, "twitter", func() send.Sender {
		twitter, err := twitter.MakeSender(ctx, &twitter.Options{
			Name:           fmt.Sprint("@", conf.Settings.Credentials.Twitter.Username, "/sardis"),
			ConsumerKey:    conf.Settings.Credentials.Twitter.ConsumerKey,
			ConsumerSecret: conf.Settings.Credentials.Twitter.ConsumerSecret,
			AccessSecret:   conf.Settings.Credentials.Twitter.OauthSecret,
			AccessToken:    conf.Settings.Credentials.Twitter.OauthToken,
		})

		if err != nil {
			err = erc.Wrap(err, "problem constructing twitter sender")
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

func DesktopNotify(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "desktop") }
func WithDesktopNotify(ctx context.Context) context.Context {
	s := desktop.MakeSender()
	s.SetName("sardis")
	sender := send.MakeMulti(s, grip.Sender())
	return grip.WithContextLogger(ctx, "desktop", grip.NewLogger(sender))
}

func RemoteNotify(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "remote-notify") }
func WithRemoteNotify(ctx context.Context, conf *Configuration) (out context.Context) {
	out = ctx

	root := grip.Sender()

	var loggers []send.Sender

	host := util.GetHostname()

	defer func() {
		desktop := desktop.MakeSender()
		desktop.SetName("sardis")

		loggers = append(loggers, desktop, root)
		sender := send.MakeMulti(loggers...)
		sender.SetPriority(level.Info)
		out = grip.WithContextLogger(ctx, "remote-notify", grip.NewLogger(sender))
	}()

	if conf.Settings.Notification.Target != "" && !conf.Settings.Notification.Disabled {
		sender, err := xmpp.NewSender(conf.Settings.Notification.Target,
			xmpp.ConnectionInfo{
				Hostname:             conf.Settings.Notification.Host,
				Username:             conf.Settings.Notification.User,
				Password:             conf.Settings.Notification.Password,
				DisableTLS:           true,
				AllowUnencryptedAuth: true,
			})

		if err != nil {
			err = erc.Wrap(err, "setting up notify send issue logger")
			srv.AddCleanupError(ctx, err)
			return
		}

		sender.SetErrorHandler(send.ErrorHandlerFromSender(root))

		sender.SetFormatter(func(m message.Composer) (string, error) {
			return fmt.Sprintf("[%s] %s", host, m.String()), nil
		})

		loggers = append(loggers, sender)

		srv.AddCleanup(ctx, func(ctx context.Context) error {
			catcher := &erc.Collector{}
			catcher.Add(sender.Flush(ctx))
			catcher.Add(sender.Close())
			return erc.Wrapf(catcher.Resolve(), "xmpp [%s]", conf.Settings.Notification.Name)
		})
	}

	if conf.Settings.Telegram.Target != "" {
		sender := send.MakeBuffered(telegram.New(conf.Settings.Telegram), time.Second, 10)

		sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
		sender.SetFormatter(func(m message.Composer) (string, error) {
			return fmt.Sprintf("[%s] %s", host, m.String()), nil
		})

		loggers = append(loggers, sender)

		srv.AddCleanup(ctx, func(ctx context.Context) error {
			catcher := &erc.Collector{}
			catcher.Add(sender.Flush(ctx))
			catcher.Add(sender.Close())
			return erc.Wrapf(catcher.Resolve(), "telegram [%s]", conf.Settings.Telegram.Name)
		})

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

	if conf.Settings.Credentials.Jira.URL == "" {
		grip.Warning("jira credentials are not configured")
		return
	}

	sender, err := jira.MakeIssueSender(ctx, &jira.Options{
		Name:    conf.Settings.Notification.Name,
		BaseURL: conf.Settings.Credentials.Jira.URL,
		BasicAuthOpts: jira.BasicAuth{
			UseBasicAuth: true,
			Username:     conf.Settings.Credentials.Jira.Username,
			Password:     conf.Settings.Credentials.Jira.Password,
		},
	})
	if err != nil {
		err = erc.Wrap(err, "setting up jira issue logger")
		srv.AddCleanupError(ctx, err)
		return
	}

	sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
	srv.AddCleanup(ctx, func(context.Context) error { return sender.Close() })
	loggers = append(loggers, sender)

	return
}
