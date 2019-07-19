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
	"os"
	"sync"
	"time"

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
	"github.com/mongodb/jasper"
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
	JiraIssue() grip.Journaler
	Jasper() jasper.Manager

	RegisterCloser(string, CloserFunc)
	Close(context.Context) error
}

type CloserFunc func(context.Context) error

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

// see the documentation for the corresponding global methods for

type appServicesCache struct {
	queue     amboy.Queue
	conf      *Configuration
	logger    grip.Journaler
	jiraIssue grip.Journaler
	jpm       jasper.Manager
	ctx       context.Context
	closers   []closerOp
	mutex     sync.RWMutex
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
	var cancel context.CancelFunc

	c.conf = conf
	c.ctx, cancel = context.WithCancel(ctx)
	c.jpm, err = jasper.NewLocalManager(true)
	catcher.Add(err)

	c.closers = append(c.closers,
		closerOp{
			name:   "cancel-app-context",
			closer: func(_ context.Context) error { cancel(); return nil },
		},
		closerOp{
			name:   "close jasper manager",
			closer: c.jpm.Close,
		},
	)

	catcher.Add(conf.Validate())
	catcher.Add(c.initSender())
	catcher.Add(c.initQueue())
	catcher.Add(c.initJira())

	return catcher.Resolve()
}

func (c *appServicesCache) initSender() error {
	sender, err := send.NewXMPPLogger(
		c.conf.Settings.Notification.Name,
		c.conf.Settings.Notification.Target,
		send.XMPPConnectionInfo{
			Hostname: c.conf.Settings.Notification.Host,
			Username: c.conf.Settings.Notification.User,
			Password: c.conf.Settings.Notification.Password,
		},
		grip.GetSender().Level())
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}

	host, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "problem finding hostname")
	}

	err = sender.SetFormatter(func(m message.Composer) (string, error) {
		return fmt.Sprintf("[%s:%s] %s", c.conf.Settings.Notification.Name, host, m.String()), nil
	})
	if err != nil {
		return errors.Wrap(err, "problem setting formatter")
	}

	c.logger = logging.MakeGrip(sender)
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

	sender, err := send.MakeJiraLogger(&send.JiraOptions{
		Name:         c.conf.Settings.Notification.Name,
		BaseURL:      c.conf.Settings.Credentials.Jira.URL,
		Username:     c.conf.Settings.Credentials.Jira.Username,
		Password:     c.conf.Settings.Credentials.Jira.Password,
		UseBasicAuth: true,
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
		closer: func(_ context.Context) error { c.queue.Runner().Close(); return nil },
	})

	return nil
}

func (c *appServicesCache) Queue() amboy.Queue {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if !c.queue.Started() {
		grip.Alert(errors.Wrap(c.queue.Start(c.ctx), "problem starting queue"))
	}

	return c.queue
}

func (c *appServicesCache) Jasper() jasper.Manager {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.Jasper()
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
			"runtime": time.Since(startAt),
			"percent": len(c.closers) / idx,
		}

		grip.LogWhen(err == nil, level.Info, msg)
		grip.Error(message.WrapError(err, msg))
	}

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
