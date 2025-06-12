package units

import (
	"context"
	"runtime"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
)

func DefaultPoolOpts() *fun.WorkerGroupConf {
	return &fun.WorkerGroupConf{
		NumWorkers:      runtime.NumCPU(),
		ContinueOnPanic: true,
		ContinueOnError: true,
	}
}

func SetupQueue[T any](op fun.Handler[T]) (*dt.List[T], fun.Worker) {
	list := &dt.List[T]{}

	return list, list.StreamFront().Parallel(op, fun.WorkerGroupConfSet(DefaultPoolOpts()))
}

func SetupWorkers(ec *erc.Collector) (*dt.List[fun.Worker], fun.Worker) {
	return SetupQueue(func(ctx context.Context, fn fun.Worker) error { err := fn(ctx); ec.Add(err); return err })
}
