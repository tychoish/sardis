/*
Package sardis holds a a number of application level constants and
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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tychoish/jasper"
	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/amboy/queue"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/logging"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

var servicesCache *appServicesCache

const SSHAgentSocketEnvVar = "SSH_AUTH_SOCK"

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

	AddCloseError(string, error)
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
	twitter    grip.Journaler
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
	catcher.Add(c.initQueue())
	catcher.Add(c.initSSHSetting(ctx))

	c.appendCloser("close-jasper", c.jpm.Close)

	return catcher.Resolve()
}

func (c *appServicesCache) initSender() error {
	root := grip.GetSender()

	var loggers []send.Sender
	defer func() {
		loggers = append(loggers, root)
		c.logger = logging.MakeGrip(send.NewConfiguredMultiSender(loggers...))
	}()
	levels := send.LevelInfo{Default: level.Notice, Threshold: level.Info}
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
	loggers = append(loggers, sender)
	c.appendCloser("sender-notify", func(_ context.Context) error { return sender.Close() })

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

	desktop, err := send.NewDesktopNotify(c.conf.Settings.Notification.Name, levels)
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}
	loggers = append(loggers, desktop)

	return nil
}

func (c *appServicesCache) initSSHSetting(ctx context.Context) error {
	if c.conf.Settings.SSHAgentSocketPath != "" {
		return nil
	}

	c.conf.Settings.SSHAgentSocketPath = os.Getenv(SSHAgentSocketEnvVar)

	if c.conf.Settings.SSHAgentSocketPath != "" {
		return nil
	}

	filepath.Walk("/tmp", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasPrefix(path, "/tmp/ssh-") {
			return nil
		}

		if c.jpm.CreateCommand(ctx).AddEnv(SSHAgentSocketEnvVar, path).AppendArgs("ssh-add", "-l").Run(ctx) == nil {
			c.conf.Settings.SSHAgentSocketPath = path
			return io.EOF // to abort early
		}

		return nil
	})

	return nil
}

func (c *appServicesCache) initJira() error {
	root := grip.GetSender()
	loggers := []send.Sender{}
	defer func() {
		loggers = append(loggers, root)
		c.jiraIssue = logging.MakeGrip(send.NewConfiguredMultiSender(loggers...))
	}()

	if c.conf.Settings.Credentials.Jira.URL == "" {
		grip.Warning("jira credentials are not configured")
		return nil
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
	loggers = append(loggers, sender)
	c.appendCloser("sender-jira-issue", func(_ context.Context) error { return sender.Close() })

	if err := sender.SetErrorHandler(send.ErrorHandlerFromSender(root)); err != nil {
		return errors.Wrap(err, "problem setting error handler")
	}

	return nil
}

func (c *appServicesCache) initTwitter() error {
	root := grip.GetSender()
	loggers := []send.Sender{}
	defer func() {
		loggers = append(loggers, root)
		c.twitter = logging.MakeGrip(send.NewConfiguredMultiSender(loggers...))
	}()

	conf := c.conf.Settings.Credentials.Twitter
	twitter, err := send.MakeTwitterLogger(c.ctx, &send.TwitterOptions{
		Name:           conf.Username + ".sardis",
		ConsumerKey:    conf.ConsumerKey,
		ConsumerSecret: conf.ConsumerSecret,
		AccessSecret:   conf.OauthSecret,
		AccessToken:    conf.OauthToken,
	})

	if err != nil {
		return errors.Wrap(err, "problem constructing twitter sender.")
	}

	if err = twitter.SetErrorHandler(send.ErrorHandlerFromSender(root)); err != nil {
		return errors.Wrap(err, "problem error handler for twitter sender.")
	}

	return nil
}

func (c *appServicesCache) initQueue() error {
	c.queue = queue.NewLocalLimitedSize(&queue.FixedSizeQueueOptions{
		Workers:  c.conf.Settings.Queue.Workers,
		Capacity: c.conf.Settings.Queue.Size,
	})

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

func (c *appServicesCache) AddCloseError(name string, err error) {
	if err == nil {
		return
	}

	c.RegisterCloser(name, func(_ context.Context) error { return err })
}

func (c *appServicesCache) addError(name string, err error) {
	if err == nil {
		return
	}

	c.appendCloser(name, func(_ context.Context) error { return err })
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

func (c *appServicesCache) appendCloser(name string, fn CloserFunc) {
	c.closers = append(c.closers, closerOp{name: name, closer: fn})
}

func (c *appServicesCache) RegisterCloser(n string, cf CloserFunc) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.appendCloser(n, cf)
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
	if c.logger != nil {
		c.mutex.RUnlock()
		return c.logger
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.initSender()

	grip.Critical(message.WrapError(err, "problem configuring notification sender"))
	c.addError("notify-constructor", err)

	return c.logger
}

func (c *appServicesCache) JiraIssue() grip.Journaler {
	c.mutex.RLock()
	if c.jiraIssue != nil {
		c.mutex.RUnlock()
		return c.logger
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.initJira()

	grip.Critical(message.WrapError(err, "problem configuring jira connection"))
	c.addError("jira-constructor", err)

	return c.jiraIssue
}

func (c *appServicesCache) Twitter() grip.Journaler {
	c.mutex.RLock()
	if c.twitter != nil {
		c.mutex.RUnlock()
		return c.twitter
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.initTwitter()

	grip.Critical(message.WrapError(err, "problem configuring twitter client"))
	c.addError("twitter-constructor", err)

	return c.jiraIssue

}
