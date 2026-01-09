package sardis

import (
	"context"

	"github.com/tychoish/fun/erc"
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
	erc.InvariantOk(value != nil, "sardis configuration is not attached to the context")

	conf, ok := value.(*Configuration)
	erc.InvariantOk(ok, "value", conf, "is a sardis app configuration type")

	erc.InvariantOk(conf != nil, "cannot return a nil configuration")
	return conf
}

func HasAppConfiguration(ctx context.Context) (ok bool) {
	_, ok = ctx.Value(confCtxKey).(*Configuration)
	return
}
