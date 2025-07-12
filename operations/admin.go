package operations

import (
	"context"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
)

func Admin() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("admin").
		SetUsage("local systems administration scripts").
		Subcommanders(
			configCheck(),
			nightly(),
			setupLinks(),
			hacking(),
		)
}

func hacking() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("hack").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				grip.Info("hackerz!")
				return nil
			}).Add)
}

func setupLinks() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("setup-links").
		SetUsage("setup all configured links").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			ec := &erc.Collector{}
			wg := &fun.WaitGroup{}

			for _, link := range conf.Links {
				wg.Launch(ctx, units.NewSymlinkCreateJob(link).Operation(ec.Push))
			}

			wg.Worker().Operation(ec.Push).Run(ctx)

			return ec.Resolve()
		}).Add)
}

func configCheck() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("config").
		SetUsage("validated configuration").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				// this is redundant, as the resolve
				// configuration does this correctly.

				err := conf.Validate()
				grip.InfoWhen(err == nil, "configuration is valid")
				return err
			}).Add)
}

func nightly() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("nightly").
		SetUsage("run nightly config operation").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			queue := pubsub.NewUnlimitedQueue[fun.Worker]()
			dist := queue.Distributor()
			ec := &erc.Collector{}

			wait := fun.MAKE.WorkerPool(dist.Stream()).Launch(ctx)

			for idx := range conf.Links {
				ec.Push(dist.Send(ctx, units.NewSymlinkCreateJob(conf.Links[idx])))
			}

			for idx := range conf.Repo {
				ec.Push(dist.Send(ctx, units.NewRepoCleanupJob(conf.Repo[idx].Path)))
			}

			for idx := range conf.System.Services {
				ec.Push(dist.Send(ctx, units.NewSystemServiceSetupJob(conf.System.Services[idx])))
			}
			queue.Close()

			return wait(ctx)
		}).Add)
}
