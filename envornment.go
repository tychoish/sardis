/*
kage sardis holds a a number of application level constants and
shared resources for the sardis application.

Services Cache

The sink package maintains a public interface to a shared cache of
interfaces and services for use in building tools within sink. The
sink package has no dependencies to any sub-packages, and all methods
in the public interface are thread safe.

In practice these values are set in the operations package. See
sink/operations/setup.go for details.
*/
package sardis

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/amboy/queue"
	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/logging"
	"github.com/deciduosity/grip/message"
	"github.com/deciduosity/grip/send"
	"github.com/deciduosity/jasper"
	"github.com/pkg/errors"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

var servicesCache *appServicesCache

func init() {
	servicesCache = &appServicesCache{}
}

func GetEnvironment() Environment {
	return servicesCache
}

type Environment interface {
	Configure(context.Context, *Configuration) error

	Context() (context.Context, context.CancelFunc)
	Configuration() *Configuration
	Queue() amboy.Queue
	Logger() grip.Journaler
	Jasper() jasper.Manager

	Twitter() grip.Journaler
	JiraIssue() grip.Journaler

	RegisterCloser(string, CloserFunc)
	Close(context.Context) error
}

type CloserFunc func(context.Context) error

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

// see the documentation for the corresponding global methods for

type appServicesCache struct {
	queue      amboy.Queue
	conf       *Configuration
	logger     grip.Journaler
	jiraIssue  grip.Journaler
	jpm        jasper.Manager
	ctx        context.Context
	rootCancel context.CancelFunc
	closers    []closerOp
	mutex      sync.RWMutex
}

type closerOp struct {
	name   string
	closer CloserFunc
}

func (c *appServicesCache) Configure(ctx context.Context, conf *Configuration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conf != nil {
		return errors.New("cannot reconfigure the environment")
	}

	catcher := grip.NewBasicCatcher()
	var err error

	c.conf = conf
	c.ctx, c.rootCancel = context.WithCancel(ctx)
	c.jpm, err = jasper.NewSynchronizedManager(false)
	catcher.Add(err)

	catcher.Add(conf.Validate())
	catcher.Add(c.initSender())
	catcher.Add(c.initQueue())
	catcher.Add(c.initJira())

	c.closers = append(c.closers,
		closerOp{
			name:   "close jasper manager",
			closer: c.jpm.Close,
		},
	)

	return catcher.Resolve()
}

func (c *appServicesCache) initSender() error {
	root := grip.GetSender()
	levels := root.Level()
	sender, err := send.NewXMPPLogger(
		c.conf.Settings.Notification.Name,
		c.conf.Settings.Notification.Target,
		send.XMPPConnectionInfo{
			Hostname: c.conf.Settings.Notification.Host,
			Username: c.conf.Settings.Notification.User,
			Password: c.conf.Settings.Notification.Password,
		},
		levels)
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}

	desktop, err := send.NewDesktopNotify(c.conf.Settings.Notification.Name, levels)
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}

	host, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "problem finding hostname")
	}

	if err = sender.SetErrorHandler(send.ErrorHandlerFromSender(root)); err != nil {
		return errors.Wrap(err, "problem setting error handler")
	}

	if err = sender.SetFormatter(func(m message.Composer) (string, error) {
		return fmt.Sprintf("[%s:%s] %s", c.conf.Settings.Notification.Name, host, m.String()), nil
	}); err != nil {
		return errors.Wrap(err, "problem setting formatter")
	}

	c.logger = logging.MakeGrip(send.NewConfiguredMultiSender(sender, desktop, root))
	c.closers = append(c.closers, closerOp{
		name:   "sender-notify",
		closer: func(_ context.Context) error { return sender.Close() },
	})
	return nil
}

func (c *appServicesCache) initJira() error {
	if c.conf.Settings.Credentials.Jira.URL == "" {
		grip.Debug("no jira instance specified, skipping jira logger")
		c.jiraIssue = logging.MakeGrip(grip.GetSender())
	}

	sender, err := send.MakeJiraLogger(c.ctx, &send.JiraOptions{
		Name:    c.conf.Settings.Notification.Name,
		BaseURL: c.conf.Settings.Credentials.Jira.URL,
		BasicAuthOpts: send.JiraBasicAuth{
			UseBasicAuth: true,
			Username:     c.conf.Settings.Credentials.Jira.Username,
			Password:     c.conf.Settings.Credentials.Jira.Password,
		},
	})
	if err != nil {
		return errors.Wrap(err, "problem setting up jira logger")
	}

	if err := sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.GetSender())); err != nil {
		return errors.Wrap(err, "problem setting error handler")
	}

	c.closers = append(c.closers, closerOp{
		name:   "sender-jira-issues",
		closer: func(_ context.Context) error { return sender.Close() },
	})

	c.jiraIssue = logging.MakeGrip(sender)
	return nil
}

func (c *appServicesCache) initQueue() error {
	c.queue = queue.NewLocalLimitedSize(c.conf.Settings.Queue.Workers, c.conf.Settings.Queue.Size)

	grip.Debug(message.Fields{
		"op":      "configured local queue",
		"size":    c.conf.Settings.Queue.Size,
		"workers": c.conf.Settings.Queue.Workers,
	})

	c.closers = append(c.closers, closerOp{
		name:   "local-queue-termination",
		closer: func(ctx context.Context) error { c.queue.Runner().Close(ctx); return nil },
	})

	return nil
}

func (c *appServicesCache) Queue() amboy.Queue {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.queue.Info().Started {
		grip.Alert(errors.Wrap(c.queue.Start(c.ctx), "problem starting queue"))
	}

	return c.queue
}

func (c *appServicesCache) Jasper() jasper.Manager {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.jpm
}

func (c *appServicesCache) Context() (context.Context, context.CancelFunc) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return context.WithCancel(c.ctx)
}

func (c *appServicesCache) Close(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	catcher := grip.NewBasicCatcher()

	for idx, op := range c.closers {
		startAt := time.Now()
		err := op.closer(ctx)
		catcher.Add(err)
		msg := message.Fields{
			"name":    op.name,
			"op":      "ran closer",
			"idx":     idx,
			"num_ops": len(c.closers),
			"runtime": time.Since(startAt),
		}

		grip.DebugWhen(err == nil, msg)
		grip.Notice(message.WrapError(err, msg))
	}

	c.rootCancel()
	return catcher.Resolve()
}

func (c *appServicesCache) RegisterCloser(n string, cf CloserFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.closers = append(c.closers, closerOp{name: n, closer: cf})
}

func (c *appServicesCache) Configuration() *Configuration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.conf == nil {
		return nil
	}

	// copy the struct
	out := Configuration{}
	out = *c.conf

	return &out
}

func (c *appServicesCache) Logger() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.logger
}

func (c *appServicesCache) JiraIssue() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.jiraIssue
}

func (c *appServicesCache) Twitter() grip.Journaler {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	conf := c.conf.Settings.Credentials.Twitter
	grip.Info(conf)
	twitter, err := send.MakeTwitterLogger(c.ctx, &send.TwitterOptions{
		Name:           conf.Username + ".sardis",
		ConsumerKey:    conf.ConsumerKey,
		ConsumerSecret: conf.ConsumerSecret,
		AccessSecret:   conf.OauthSecret,
		AccessToken:    conf.OauthToken,
	})
	if err != nil {
		grip.Critical(message.WrapError(err, "problem constructing twitter sender."))
		return c.logger
	}

	return logging.MakeGrip(send.NewConfiguredMultiSender(twitter, grip.GetSender()))
}
