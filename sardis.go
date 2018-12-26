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

	"github.com/mongodb/amboy"
	"github.com/mongodb/amboy/queue"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/logging"
	"github.com/mongodb/grip/message"
	"github.com/mongodb/grip/send"
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

	Configuration() *Configuration
	Queue() amboy.Queue
	Logger() grip.Journaler
}

////////////////////////////////////////////////////////////////////////
//
// internal implementation of the cache

// see the documentation for the corresponding global methods for

type appServicesCache struct {
	queue  amboy.Queue
	conf   *Configuration
	logger grip.Journaler

	mutex sync.RWMutex
}

func (c *appServicesCache) Configure(ctx context.Context, conf *Configuration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.conf = conf

	catcher := grip.NewBasicCatcher()

	catcher.Add(c.initSender())
	catcher.Add(c.initQueue(ctx))

	return catcher.Resolve()
}

func (c *appServicesCache) initSender() error {
	sender, err := send.NewXMPP("sardis", c.conf.Notification.Target, grip.GetSender().Level())
	if err != nil {
		return errors.Wrap(err, "problem creating sender")
	}

	host, err := os.Hostname()
	if err != nil {
		return errors.Wrap(err, "problem finding hostname")
	}

	sender.SetFormatter(func(m message.Composer) (string, error) {
		return fmt.Sprintf("[sardis:%s] %s", host, m.String()), nil
	})

	c.logger = logging.MakeGrip(sender)

	return nil
}

func (c *appServicesCache) initQueue(ctx context.Context) error {
	c.queue = queue.NewLocalLimitedSize(c.conf.Queue.Workers, c.conf.Queue.Size)

	grip.Debug(message.Fields{
		"op":      "configured local queue",
		"size":    c.conf.Queue.Size,
		"workers": c.conf.Queue.Workers,
	})

	return errors.Wrap(c.queue.Start(ctx), "problem starting queue")
}

func (c *appServicesCache) Queue() amboy.Queue {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.queue
}

func (c *appServicesCache) Configuration() *Configuration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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
