package units

import (
	"context"
	"runtime"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
)

func DefaultPoolOpts() itertool.Options {
	return itertool.Options{
		NumWorkers:      runtime.NumCPU(),
		ContinueOnPanic: true,
		ContinueOnError: true,
	}
}

func SetupQueue[T any](op func(context.Context, T) error) (*seq.List[T], fun.WorkerFunc) {
	list := &seq.List[T]{}

	return list, func(ctx context.Context) error {
		return itertool.ParallelForEach(ctx, seq.ListValues(list.Iterator()), op, DefaultPoolOpts())
	}
}

func SetupWorkers() (*seq.List[fun.WorkerFunc], fun.WorkerFunc) {
	return SetupQueue(func(ctx context.Context, fn fun.WorkerFunc) error { return fn(ctx) })
}
