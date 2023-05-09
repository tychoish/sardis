package sardis

import (
	"context"

	"github.com/tychoish/grip"
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

func AppConfiguration(ctx context.Context) *Configuration {
	val, ok := ctx.Value(confCtxKey).(Configuration)
	if !ok {
		grip.Critical("configuration not loaded in context")
		return nil
	}
	return &val
}

func HasAppConfiguration(ctx context.Context) bool {
	_, ok := ctx.Value(confCtxKey).(Configuration)
	return ok
}
