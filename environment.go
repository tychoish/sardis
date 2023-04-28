/*
Package sardis holds a a number of application level constants and
shared resources for the sardis application.

# Services Cache

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
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/grip/x/xmpp"
	"github.com/tychoish/jasper"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

const SSHAgentSocketEnvVar = "SSH_AUTH_SOCK"

type envCtxKey struct{}

func GetEnvironment(ctx context.Context) Environment {
	val := ctx.Value(envCtxKey{})
	fun.Invariant(val != nil, "environment must non-nil")

	env, ok := val.(Environment)
	fun.Invariant(ok, "environment context key must be of the correct type")

	return env
}

func WithEvironment(ctx context.Context, env Environment) context.Context {
	return context.WithValue(ctx, envCtxKey{}, env)
}

type Environment interface {
	Configuration() *Configuration
	Jasper() jasper.Manager

	Logger() grip.Logger

	Close(context.Context) error
}

type CloserFunc func(context.Context) error

func NewEnvironment(ctx context.Context, conf *Configuration) (Environment, error) {
	env := &appServicesCache{}
	err := env.Configure(ctx, conf)
	if err != nil {
		return nil, err
	}
	return env, nil
}

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

// see the documentation for the corresponding global methods for

type appServicesCache struct {
	mutex sync.RWMutex

	conf       *Configuration
	notifySend grip.Logger
	jpm        jasper.Manager

	closers seq.List[fun.WorkerFunc]
}

func (c *appServicesCache) Configure(ctx context.Context, conf *Configuration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conf != nil {
		return errors.New("cannot reconfigure the environment")
	}

	catcher := &erc.Collector{}
	var err error
	c.conf = conf
	c.jpm, err = jasper.NewSynchronizedManager(false)
	catcher.Add(err)

	catcher.Add(conf.Validate())

	c.appendCloser("close-jasper", c.jpm.Close)

	return catcher.Resolve()
}

func (c *appServicesCache) initSender() error {
	root := grip.Sender()

	var loggers []send.Sender
	defer func() {
		loggers = append(loggers, root)
		c.notifySend = grip.NewLogger(send.MakeMulti(loggers...))
	}()
	sender, err := xmpp.NewSender(
		c.conf.Settings.Notification.Target,
		xmpp.ConnectionInfo{
			Hostname:             c.conf.Settings.Notification.Host,
			Username:             c.conf.Settings.Notification.User,
			Password:             c.conf.Settings.Notification.Password,
			DisableTLS:           true,
			AllowUnencryptedAuth: true,
		})
	if err != nil {
		return fmt.Errorf("problem creating sender: %w", err)
	}
	sender.SetPriority(level.Info)
	sender.SetName(c.conf.Settings.Notification.Name)
	loggers = append(loggers, sender)
	c.appendCloser("sender-notify", func(ctx context.Context) error {
		catcher := &erc.Collector{}
		catcher.Add(sender.Flush(ctx))
		catcher.Add(sender.Close())
		return catcher.Resolve()
	})

	host, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("problem finding hostname: %w", err)
	}

	sender.SetErrorHandler(send.ErrorHandlerFromSender(root))
	sender.SetFormatter(func(m message.Composer) (string, error) {
		return fmt.Sprintf("[%s:%s] %s", c.conf.Settings.Notification.Name, host, m.String()), nil
	})

	c.notifySend = grip.NewLogger(send.NewMulti("sardis", loggers))

	return nil
}

func (c *appServicesCache) Jasper() jasper.Manager {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.jpm
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
	start := time.Now()

	catcher := &erc.Collector{}
	itertool.ParallelForEach(ctx,
		seq.ListValues(c.closers.Iterator()),
		func(ctx context.Context, wf fun.WorkerFunc) error { return wf.Run(ctx) },
		itertool.Options{
			ContinueOnPanic: true,
			ContinueOnError: true,
			NumWorkers:      runtime.NumCPU(),
		},
	)

	grip.Debug(message.Fields{
		"op":       "run all closers",
		"num":      c.closers.Len(),
		"duration": time.Since(start),
	})

	return catcher.Resolve()
}

func (c *appServicesCache) appendCloser(name string, fn CloserFunc) {
	c.closers.PushBack(func(ctx context.Context) error {
		startAt := time.Now()
		err := fn(ctx)
		msg := message.Fields{
			"name":    name,
			"op":      "ran closer",
			"runtime": time.Since(startAt),
		}

		grip.DebugWhen(err == nil, msg)
		grip.Notice(message.WrapError(err, msg))
		return err
	})
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

func (c *appServicesCache) Logger() grip.Logger {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.notifySend.Sender() != nil {
		return c.notifySend
	}
	err := c.initSender()

	grip.Critical(message.WrapError(err, "problem configuring notification sender"))
	c.addError("notify-constructor", err)
	return c.notifySend
}
