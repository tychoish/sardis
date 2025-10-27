package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"slices"
	"strconv"

	"github.com/cheynewallace/tabby"
	"github.com/mattn/go-isatty"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/global"
	"github.com/tychoish/sardis/repo"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/sysmgmt"
	"github.com/tychoish/sardis/util"
)

func Admin() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("admin").
		SetUsage("local systems administration scripts").
		Subcommanders(
			configCheck(),
			nightly(),
			linkOp(),
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
				"istty":                      isatty.IsTerminal(os.Stdin.Fd()),
				"version":                    sardis.BuildRevision,
				"alacritty":                  conf.Operations.AlacrittySocket(),
				"ssh_agent":                  conf.Operations.SSHAgentSocket(),
				"ops.include_local":          conf.Operations.Settings.IncludeLocalSHH,
				global.EnvVarAlacrittySocket: os.Getenv(global.EnvVarAlacrittySocket),
				global.EnvVarSSHAgentSocket:  os.Getenv(global.EnvVarSSHAgentSocket),
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

func linkOp() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("links").
		Aliases("setup-links").
		SetUsage("setup all configured links").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				links := fun.SliceStream(conf.System.Links.Links)

				jobs := fun.MakeConverter(func(c sysmgmt.LinkDefinition) fun.Worker { return c.CreateLinkJob() }).Stream(links)

				return subexec.TOOLS.WorkerPool(jobs).Run(ctx)
			}).
			Add).
		Subcommanders(addOpCommand(cmdr.MakeCommander().
			SetName("discover").
			Aliases("disco", "disc").SetUsage("discover"),
			"path", func(ctx context.Context, args *withConf[string]) error {
				var idx int

				ec := &erc.Collector{}
				table := tabby.New()
				table.AddHeader("Index", "+sudo", "Target", "Path")
				args.conf.System.Links.Discovery.SearchPaths = append(args.conf.System.Links.Discovery.SearchPaths, args.arg)
				args.conf.System.Links.Discovery.FindLinks().ReadAll(func(d *sysmgmt.LinkDefinition) {
					table.AddLine(
						strconv.Itoa(idx),
						ft.IfElse(d.RequireSudo, "sudo", "-"),
						util.TryCollapseHomeDir(d.Target),
						util.TryCollapseHomeDir(d.Path),
					)
					idx++
				}).Operation(ec.Push).Run(ctx)
				table.Print()

				return ec.Resolve()
			}))
}

func configCheck() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("config").
		Aliases("conf").
		SetUsage("validated configuration").
		Subcommanders(addOpCommand(cmdr.MakeCommander().
			SetName("system").
			Aliases("sys"),
			"para", func(ctx context.Context, args *withConf[string]) error {
				ec := &erc.Collector{}

				buf := bufio.NewWriter(os.Stdout)
				enc := json.NewEncoder(buf)
				enc.SetIndent("", "    ")

				ec.Push(enc.Encode(args.conf.System.SystemD))
				ec.Push(buf.Flush())

				return ec.Resolve()
			})).
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				ec := &erc.Collector{}

				buf := bufio.NewWriter(os.Stdout)
				enc := json.NewEncoder(buf)
				enc.SetIndent("", "    ")

				ec.Push(enc.Encode(conf))
				ec.Push(buf.Flush())

				return ec.Resolve()
			}).Add)
}

func nightly() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("nightly").
		SetUsage("run nightly config operation").
		With(cmdr.SpecBuilder(
			ResolveConfiguration,
		).SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
			jobs := fun.JoinStreams(
				fun.MakeConverter(func(c sysmgmt.LinkDefinition) fun.Worker { return c.CreateLinkJob() }).Stream(fun.SliceStream(conf.System.Links.Links)),
				fun.MakeConverter(func(c repo.GitRepository) fun.Worker { return c.CleanupJob() }).Stream(fun.SliceStream(conf.Repos.GitRepos)),
				fun.MakeConverter(func(c sysmgmt.SystemdService) fun.Worker { return c.Worker() }).Stream(fun.SliceStream(conf.System.SystemD.Services)),
			)

			return subexec.TOOLS.WorkerPool(jobs).Run(ctx)
		}).Add)
}
