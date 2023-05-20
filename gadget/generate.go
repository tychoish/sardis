package gadget

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

func RunCommand(ctx context.Context, buildSpec *BuildOrder, workers int, args []string) error {
	main := pubsub.NewUnlimitedQueue[fun.WorkerFunc]()
	pool := srv.WorkerPool(main, itertool.Options{NumWorkers: workers, ContinueOnError: true})

	if err := pool.Start(ctx); err != nil {
		return err
	}

	out := send.MakeWriter(send.MakePlain())
	out.SetPriority(grip.Sender().Priority())

	jpm := jasper.Context(ctx)
	index := buildSpec.Packages.IndexByPackageName()

	opStart := time.Now()
	for groupIdx, group := range buildSpec.Order {
		gwg := &fun.WaitGroup{}
		gStart := time.Now()
		for _, pkg := range group {
			info := index[pkg]
			gwg.Add(1)
			main.Add(jpm.CreateCommand(ctx).
				ID(fmt.Sprint("generate.", pkg)).
				Directory(info.LocalDirectory).
				PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
				SetOutputSender(level.Debug, out).
				SetErrorSender(level.Error, out).
				PostHook(func(err error) error {
					gwg.Done()
					return err
				}).
				AppendArgs(args...).
				Run)
		}
		gwg.Wait(ctx)
		grip.Info(message.BuildPair().
			Pair("op", "run group command").
			Pair("dur", time.Since(gStart)).
			Pair("group", groupIdx+1).
			Pair("size", len(group)))
	}

	grip.Notice(message.BuildPair().
		Pair("op", "run command").
		Pair("dur", time.Since(opStart)).
		Pair("cmd", strings.Join(args, " ")).
		Pair("size", len(buildSpec.Order)))

	pool.Close()

	return pool.Wait()
}
