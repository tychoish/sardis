package units

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
)

func SetupQueue[T any](op fun.Handler[T]) (*dt.List[T], fun.Worker) {
	list := &dt.List[T]{}

	return list, list.StreamPopFront().Parallel(op, fun.WorkerGroupConfWorkerPerCPU())
}

func SetupWorkers() (*dt.List[fun.Worker], fun.Worker) {
	return SetupQueue(func(ctx context.Context, fn fun.Worker) error { return fn.Run(ctx) })
}
