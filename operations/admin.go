package operations

import (
	"context"
	"os"
	"slices"

	"github.com/cheynewallace/tabby"
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
			grip.Noticeln("has default jasper", jasper.HasManager(ctx))

			grip.Info(message.Fields{
				"version":                    sardis.BuildRevision,
				"alacritty":                  conf.Operations.AlacrittySocket(),
				"ssh_agent":                  conf.Operations.SSHAgentSocket(),
				"ops.include_local":          conf.Operations.Settings.IncludeLocalSHH,
				"runtime.include_local":      conf.Settings.Runtime.IncludeLocalSHH,
				"ops.hostname":               conf.Operations.Settings.Hostname,
				"runtime.hostname":           conf.Settings.Runtime.Hostname,
				sardis.EnvVarAlacrittySocket: os.Getenv(sardis.EnvVarAlacrittySocket),
				sardis.EnvVarSSHAgentSocket:  os.Getenv(sardis.EnvVarSSHAgentSocket),
			})
			table := tabby.New()
			table.AddHeader("Group", "Command")
			for cg := range slices.Values(conf.Operations.Commands) {
				for i := 0; i < len(cg.Commands); i++ {
					if i == 0 {
						table.AddLine(cg.Name, cg.NamesAtIndex(i)[0:])
						continue
					}
					table.AddLine("", cg.NamesAtIndex(i)[0:])
				}
				table.AddLine("", "")
			}
			table.Print()
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

			for idx := range conf.Repos.GitRepos {
				ec.Push(dist.Send(ctx, units.NewRepoCleanupJob(conf.Repos.GitRepos[idx])))
			}

			for idx := range conf.System.Services {
				ec.Push(dist.Send(ctx, units.NewSystemServiceSetupJob(conf.System.Services[idx])))
			}
			queue.Close()

			return wait(ctx)
		}).Add)
}
