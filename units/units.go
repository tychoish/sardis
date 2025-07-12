package units

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/pubsub"
)

func SetupQueue[T any](op fun.Handler[T]) (*pubsub.Queue[T], fun.Worker) {
	queue := pubsub.NewUnlimitedQueue[T]()

	return queue, queue.Distributor().Stream().
		Parallel(op, fun.WorkerGroupConfWorkerPerCPU())
}

func SetupWorkers() (*pubsub.Queue[fun.Worker], fun.Worker) {
	return SetupQueue(func(ctx context.Context, fn fun.Worker) error { return fn.Run(ctx) })
}
