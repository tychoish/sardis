package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"iter"
	"os"
	"slices"

	"github.com/cheynewallace/tabby"
	"github.com/mattn/go-isatty"
	"github.com/tychoish/birch/jsonx"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/global"
	srsrv "github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/sysmgmt"
	"github.com/tychoish/sardis/util"
	"github.com/urfave/cli/v3"
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
		Subcommanders(
			cmdr.MakeCommander().
				SetName("pkg").
				With(StandardSardisOperationSpec().SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
					seq := conf.System.Arch.ResolvePackages(ctx)
					for pkg := range seq {
						grip.Info(pkg)
					}
					return nil
				}).Add),
			cmdr.MakeCommander().
				SetName("env").
				With(StandardSardisOperationSpec().SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
					grip.Notice(grip.MPrintln("has conf", conf != nil))
					grip.Notice(grip.MPrintln("has default jasper", jasper.HasManager(ctx)))

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
				}).Add))
}

func linkOp() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("links").
		Aliases("setup-links").
		SetUsage("setup all configured links").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				workers := func(yield func(fnx.Worker) bool) {
					for _, link := range conf.System.Links.Links {
						if !yield(link.CreateLinkJob()) {
							return
						}
					}
				}
				return subexec.TOOLS.WorkerPool(workers).Run(ctx)
			}).
			Add).
		Subcommanders(cmdr.MakeCommander().
			SetName("discover").
			Aliases("disco", "disc").
			SetUsage("discover symlinks on the filesystem").
			Flags(
				cmdr.FlagBuilder("table").
					SetName("format", "f").
					SetUsage("output format: table|json|yaml|yaml-list|line").
					Flag(),
				cmdr.FlagBuilder(false).
					SetName("skip-defined").
					SetUsage("show only problems: exists-but-undefined and defined-but-missing").
					Flag(),
			).
			With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Command) (*discoverArgs, error) {
				conf, err := ResolveConfiguration(ctx, cc)
				if err != nil {
					return nil, err
				}
				format := cc.String("format")
				if !cc.IsSet("format") {
					if first := cc.Args().First(); first != "" {
						format = first
					}
				}
				return &discoverArgs{
					conf:        conf,
					format:      format,
					skipDefined: cc.Bool("skip-defined"),
				}, nil
			}).SetMiddleware(func(ctx context.Context, args *discoverArgs) context.Context {
				ctx = sardis.WithConfiguration(ctx, args.conf)
				ctx = subexec.WithJasper(ctx, &args.conf.Operations)
				ctx = srsrv.WithAppLogger(ctx, args.conf.Settings.Logging)
				ctx = srsrv.WithRemoteNotify(ctx, args.conf.Settings)
				return ctx
			}).SetAction(func(ctx context.Context, args *discoverArgs) error {
				if args.conf.System.Links.Discovery == nil {
					return errors.New("discovery config not defined")
				}
				ec := &erc.Collector{}
				lookup := args.conf.System.Links.Resolve()

				links := func(yield func(sysmgmt.LinkDefinition) bool) {
					seen := map[string]struct{}{}
					for d := range args.conf.System.Links.Discovery.FindLinks() {
						d.Defined = lookup.Check(d.Path)
						seen[d.Path] = struct{}{}
						if args.skipDefined && d.Defined && d.PathExists {
							continue
						}
						if !yield(d) {
							return
						}
					}
					if args.skipDefined {
						for _, link := range args.conf.System.Links.Links {
							if _, ok := seen[link.Path]; ok {
								continue
							}
							link.Defined = true
							link.PathExists = util.FileExists(link.Path)
							link.TargetExists = util.FileExists(link.Target)
							if link.PathExists {
								continue
							}
							if !yield(link) {
								return
							}
						}
					}
				}

				switch args.format {
				case "JSON", "json", "js", "j":
					buf := bufio.NewWriter(os.Stdout)
					for d := range links {
						erc.Must(buf.Write(erc.Must(jsonx.DC.Elements(
							jsonx.EC.String("path", d.Path),
							jsonx.EC.String("target", d.Target),
							jsonx.EC.Boolean("defined", d.Defined),
							jsonx.EC.Boolean("target_exists", d.TargetExists),
							jsonx.EC.Boolean("path_exists", d.PathExists),
						).MarshalJSON())))
						ec.Push(buf.WriteByte('\n'))
					}
					ec.Push(buf.Flush())
				case "line", "ln", "print":
					buf := bufio.NewWriter(os.Stdout)
					enc := yaml.NewEncoder(buf)
					type linksConf struct {
						Links []sysmgmt.LinkDefinition `yaml:"links"`
					}
					out := linksConf{}
					for d := range links {
						out.Links = append(out.Links, d)
					}
					ec.Push(enc.Encode(out))
					ec.Push(enc.Close())
					ec.Push(buf.Flush())
				case "yaml-list", "yl":
					buf := bufio.NewWriter(os.Stdout)
					enc := yaml.NewEncoder(buf)
					ec.Push(enc.Encode(irt.Collect(links)))
					ec.Push(enc.Close())
					ec.Push(buf.Flush())
				case "YAML", "yaml", "yml", "y", "export":
					buf := bufio.NewWriter(os.Stdout)
					enc := yaml.NewEncoder(buf)
					for d := range links {
						ec.Push(enc.Encode(d))
					}
					ec.Push(enc.Close())
					ec.Push(buf.Flush())
				case "table":
					fallthrough
				default:
					table := tabby.New()
					table.AddHeader("Path", "Target", "Exists", "Defined")
					items := irt.Collect(links)
					slices.SortFunc(items, func(a, b sysmgmt.LinkDefinition) int {
						if a.LessThan(b) {
							return -1
						}
						if b.LessThan(a) {
							return 1
						}
						return 0
					})
					for _, d := range items {
						table.AddLine(
							util.TryCollapseHomeDir(d.Path),
							util.TryCollapseHomeDir(d.Target),
							renderBool(d.TargetExists),
							renderBool(d.Defined),
						)
					}
					table.Print()
				}
				return ec.Resolve()
			}).Add))
}

type discoverArgs struct {
	conf        *sardis.Configuration
	format      string
	skipDefined bool
}

func renderBool(in bool) string {
	if in {
		return "t"
	} else {
		return "f"
	}
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
			workers := func(yield func(fnx.Worker) bool) {
				for _, link := range conf.System.Links.Links {
					if !yield(link.CreateLinkJob()) {
						return
					}
				}
				for _, repo := range conf.Repos.GitRepos {
					if !yield(repo.CleanupJob()) {
						return
					}
				}
				for _, service := range conf.System.SystemD.Services {
					if !yield(service.Worker()) {
						return
					}
				}
			}
			return subexec.TOOLS.WorkerPool(workers).Run(ctx)
		}).Add)
}

func Set[T comparable](it iter.Seq[T]) map[T]struct{} {
	set := make(map[T]struct{})
	for val := range it {
		set[val] = struct{}{}
	}
	return set
}

func containsAny[T comparable](it iter.Seq[T], vals ...T) bool {
	for value := range it {
		for check := range slices.Values(vals) {
			if value == check {
				return true
			}
		}
	}
	return false
}
