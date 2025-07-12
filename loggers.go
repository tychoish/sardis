package sardis

import (
	"context"
	"fmt"
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
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/desktop"
	"github.com/tychoish/grip/x/jira"
	"github.com/tychoish/grip/x/system"
	"github.com/tychoish/grip/x/telegram"
	"github.com/tychoish/grip/x/twitter"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/jasper/util"
)

func WithAppLogger(ctx context.Context, conf *Configuration) context.Context {
	sender := SetupLogging(conf)
	grip.SetSender(sender)
	return grip.WithLogger(ctx, grip.NewLogger(sender))
}

func SetupLogging(conf *Configuration) send.Sender {
	var sender send.Sender

	if conf.Settings.Logging.EnableJSONFormating || conf.Settings.Logging.EnableJSONColorFormatting {
		sender = send.MakePlain()
	} else {
		sender = grip.Sender()
	}

	if runtime.GOOS == "linux" && !conf.Settings.Logging.DisableSyslog && journal.Enabled() {
		syslog := system.MakeDefault()
		syslog.SetName(filepath.Base(os.Args[0]))

		if conf.Settings.Logging.DisableStandardOutput {
			sender = syslog
		} else {
			sender = send.MakeMulti(syslog, sender)
		}
	}

	switch {
	case conf.Settings.Logging.EnableJSONColorFormatting:
		sender.SetFormatter(func(m message.Composer) (string, error) {
			out, err := jsoncolor.Marshal(m.Raw())
			if err != nil {
				return "", err
			}
			return string(out), nil
		})
	case conf.Settings.Logging.EnableJSONFormating:
		sender.SetFormatter(send.MakeJSONFormatter())
	}

	sender.SetPriority(conf.Settings.Logging.Priority)
	sender.SetName(filepath.Base(os.Args[0]))

	return sender
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

func DesktopNotify(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "desktop") }
func WithDesktopNotify(ctx context.Context) context.Context {
	root := grip.Sender()
	s := desktop.MakeSender()
	s.SetName("sardis")
	s.SetPriority(root.Priority())
	sender := send.MakeMulti(s, root)
	return grip.WithContextLogger(ctx, "desktop", grip.NewLogger(sender))
}

func RemoteNotify(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "remote-notify") }
func WithRemoteNotify(ctx context.Context, conf *Configuration) (out context.Context) {
	var loggers []send.Sender
	root := grip.Sender()
	defer func() {
		out = grip.WithContextLogger(ctx, "remote-notify",
			grip.NewLogger(send.NewMulti(
				root.Name(),
				append(loggers, root),
			)))
	}()

	host := util.GetHostname()

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
			return ers.Wrapf(catcher.Resolve(), "xmpp [%s]", conf.Settings.Notification.Name)
		})
	}

	if conf.Settings.Telegram.Target != "" {
		sender := send.MakeBuffered(telegram.New(conf.Settings.Telegram), time.Second, 10)
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
			return ers.Wrapf(catcher.Resolve(), "telegram [%s]", conf.Settings.Telegram.Name)
		})

	}
	if len(loggers) == 0 {
		desktop := desktop.MakeSender()
		desktop.SetName("sardis")
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
		srv.AddCleanupError(ctx, ers.Wrap(err, "setting up jira issue logger"))
		return
	}

	sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
	srv.AddCleanup(ctx, func(context.Context) error { return sender.Close() })
	loggers = append(loggers, sender)

	return
}
