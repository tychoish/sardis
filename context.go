package sardis

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
)

type ctxKey struct{}

func (ctxKey) String() string { return "sardis.conf" }

var confCtxKey struct{}

func WithConfiguration(ctx context.Context, conf *Configuration) context.Context {
	if HasAppConfiguration(ctx) {
		return ctx
	}
	return context.WithValue(ctx, confCtxKey, conf)
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
	return ft.IsType[*Configuration](ctx.Value(confCtxKey))
}
