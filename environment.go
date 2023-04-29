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
	"sync"

	"github.com/tychoish/fun"
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
}

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

	conf *Configuration
}

func (c *appServicesCache) Configure(ctx context.Context, conf *Configuration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.conf != nil {
		return errors.New("cannot reconfigure the environment")
	}
	var err error
	c.conf = conf
	return err
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
