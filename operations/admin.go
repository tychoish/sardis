package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/cheynewallace/tabby"
	"github.com/mattn/go-isatty"
	"github.com/tychoish/birch/jsonx"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
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
	"gopkg.in/yaml.v2"
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
				return subexec.TOOLS.WorkerPool(fun.Convert(fnx.MakeConverter(func(c sysmgmt.LinkDefinition) fnx.Worker {
					return c.CreateLinkJob()
				})).Stream(
					fun.SliceStream(conf.System.Links.Links),
				))
			}).
			Add).
		Subcommanders(addOpCommand(cmdr.MakeCommander().
			SetName("discover").
			Aliases("disco", "disc").SetUsage("discover"),
			"format", func(ctx context.Context, args *withConf[string]) error {
				if args.conf.System.Links.Discovery == nil {
					return errors.New("discovery config not defined")
				}
				ec := &erc.Collector{}

				switch args.arg {
				case "JSON", "json":
					buf := bufio.NewWriter(os.Stdout)
					args.conf.System.Links.Discovery.FindLinks().ReadAll(fnx.FromHandler(func(d *sysmgmt.LinkDefinition) {
						ec.Push(ft.IgnoreFirst(buf.Write(ft.IgnoreSecond(jsonx.DC.Elements(
							jsonx.EC.String("path", d.Path),
							jsonx.EC.String("target", d.Target),
						).MarshalJSON()))))
						ec.Push(buf.WriteByte('\n'))
					})).Operation(ec.Push).Run(ctx)
					ec.Push(buf.Flush())
				case "line", "ln":
					buf := bufio.NewWriter(os.Stdout)
					args.conf.System.Links.Discovery.FindLinks().ReadAll(func(ctx context.Context, d *sysmgmt.LinkDefinition) error {
						return ft.IgnoreFirst(fmt.Fprintln(buf, d.Path, "->", d.Target))
					}).Operation(ec.Push).Run(ctx)
					ec.Push(buf.Flush())
				case "yaml", "export":
					buf := bufio.NewWriter(os.Stdout)
					enc := yaml.NewEncoder(buf)

					args.conf.System.Links.Discovery.FindLinks().ReadAll(func(ctx context.Context, d *sysmgmt.LinkDefinition) error {
						return enc.Encode(d)
					}).Operation(ec.Push).Run(ctx)

					ec.Push(enc.Close())
					ec.Push(buf.Flush())
				case "table":
					fallthrough
				default:
					table := tabby.New()
					table.AddHeader("Path", "", "Target")

					args.conf.System.Links.Discovery.FindLinks().ReadAll(fnx.FromHandler(func(d *sysmgmt.LinkDefinition) {
						table.AddLine(
							util.TryCollapseHomeDir(d.Path),
							"=>",
							util.TryCollapseHomeDir(d.Target),
						)
					})).Operation(ec.Push).Run(ctx)
					table.Print()
				}

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
			return subexec.TOOLS.WorkerPool(fun.JoinStreams(
				fun.Convert(fnx.MakeConverter(func(c sysmgmt.LinkDefinition) fnx.Worker { return c.CreateLinkJob() })).Stream(fun.SliceStream(conf.System.Links.Links)),
				fun.Convert(fnx.MakeConverter(func(c repo.GitRepository) fnx.Worker { return c.CleanupJob() })).Stream(fun.SliceStream(conf.Repos.GitRepos)),
				fun.Convert(fnx.MakeConverter(func(c sysmgmt.SystemdService) fnx.Worker { return c.Worker() })).Stream(fun.SliceStream(conf.System.SystemD.Services)),
			)).Run(ctx)
		}).Add)
}
