package operations

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
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
	return cmdr.MakeCommander().
		SetName("hack").
		With(StandardSardisOperationSpec().SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			grip.Noticeln("has conf", conf != nil)
			grip.Noticeln("has jasper", jasper.HasManager(ctx))

			grip.Info(message.Fields{
				"version":                    sardis.BuildRevision,
				"alacritty":                  conf.AlacrittySocket(),
				"ssh_agent":                  conf.SSHAgentSocket(),
				sardis.EnvVarAlacrittySocket: os.Getenv(sardis.EnvVarAlacrittySocket),
				sardis.EnvVarSSHAgentSocket:  os.Getenv(sardis.EnvVarSSHAgentSocket),
			})
			for cg := range slices.Values(conf.Commands) {
				fmt.Println("START GROUP", cg.Name, "--------")
				for i := 0; i < len(cg.Commands); i++ {
					fmt.Println("--- ", strings.Join(cg.NamesAtIndex(i), "\n     "))
				}
				fmt.Println("END GROUP", cg.Name, "---------")
			}

			return nil
		}).Add)

}

func setupLinks() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("setup-links").
		SetUsage("setup all configured links").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
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
		With(StandardSardisOperationSpec().
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
				ec.Push(dist.Send(ctx, units.NewRepoCleanupJob(conf.Repo[idx])))
			}

			for idx := range conf.System.Services {
				ec.Push(dist.Send(ctx, units.NewSystemServiceSetupJob(conf.System.Services[idx])))
			}
			queue.Close()

			return wait(ctx)
		}).Add)
}
