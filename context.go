package sardis

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
)

type ctxKey string

const confCtxKey ctxKey = "sardis-conf"

type ContextSetupFunction[T any] func(context.Context, T) context.Context

func ContextSetup[T any](fns ...ContextSetupFunction[T]) ContextSetupFunction[T] {
	return func(ctx context.Context, conf T) context.Context {
		for _, fn := range fns {
			ctx = fn(ctx, conf)
		}
		return ctx
	}
}

func WithConfiguration(ctx context.Context, conf *Configuration) context.Context {
	if HasAppConfiguration(ctx) {
		return ctx
	}
	return context.WithValue(ctx, confCtxKey, *conf)
}

func WithJasper(ctx context.Context, conf *Configuration) context.Context {
	jpm := jasper.NewManager(
		jasper.ManagerOptionSetSynchronized(),
		jasper.ManagerOptionWithEnvVar(EnvVarAlacrittySocket, conf.Operations.AlacrittySocket()),
		jasper.ManagerOptionWithEnvVar(EnvVarSSHAgentSocket, conf.Operations.SSHAgentSocket()),
	)
	srv.AddCleanup(ctx, jpm.Close)

	noStdOut := jasper.NewManager(
		jasper.ManagerOptionSetSynchronized(),
		jasper.ManagerOptionWithEnvVar(EnvVarAlacrittySocket, conf.Operations.AlacrittySocket()),
		jasper.ManagerOptionWithEnvVar(EnvVarSSHAgentSocket, conf.Operations.SSHAgentSocket()),
		jasper.ManagerOptionWithEnvVar(EnvVarSardisLogQuietStdOut, "true"),
	)
	srv.AddCleanup(ctx, noStdOut.Close)

	jasper.WithContextManager(ctx, "without-std-out", noStdOut)
	return jasper.WithManager(ctx, jpm)
}

func AppConfiguration(ctx context.Context) *Configuration {
	val, ok := ctx.Value(confCtxKey).(*Configuration)
	if !ok {
		grip.Critical("configuration not loaded in context")
		return nil
	}

	if val == nil {
		grip.Alert("found nil configuration in context")
		return nil
	}
	return val
}

func MustAppConfiguration(ctx context.Context) *Configuration {
	value := ctx.Value(confCtxKey)
	fun.Invariant.IsFalse(ft.IsNil(value), "sardis configuration is not attached to the context")

	conf, ok := value.(*Configuration)
	fun.Invariant.Ok(ok, "value", conf, "is a sardis app configuration type")

	fun.Invariant.IsTrue(conf != nil, "cannot return a nil configuration")
	return conf
}

func HasAppConfiguration(ctx context.Context) bool {
	return ft.IgnoreFirst(ft.Cast[*Configuration](ctx.Value(confCtxKey)))
}
