package sardis

import (
	"context"
	"fmt"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/desktop"
	"github.com/tychoish/grip/x/jira"
	"github.com/tychoish/grip/x/twitter"
)

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
	return grip.WithNewContextLogger(ctx, "desktop", func() send.Sender {
		desktop := desktop.MakeSender()
		desktop.SetName(grip.Sender().Name())
		return desktop
	})
}

func JiraIssue(ctx context.Context) grip.Logger { return grip.ContextLogger(ctx, "jira-issue") }
func WithJiraIssue(ctx context.Context, conf *Configuration) (out context.Context) {
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
		erc.Wrap(err, "setting up jira issue logger")
		grip.Alert(err)
		return
	}
	srv.AddCleanupError(ctx, err)

	sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
	srv.AddCleanup(ctx, func(context.Context) error { return sender.Close() })
	loggers = append(loggers, sender)

	return
}
