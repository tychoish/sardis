package operations

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	qrcodeTerminal "github.com/Baozisoftware/qrcode-terminal-go"
	"github.com/cheynewallace/tabby"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
	sutil "github.com/tychoish/sardis/util"
)

const commandFlagName = "command"

func RunCommand() *cmdr.Commander {
	cmd := cmdr.MakeCommander().SetName("run").
		SetUsage("runs a predefined command").
		Subcommanders(
			listCommands(),
			dmenuCommand(dmenuCommandAll).SetName("dmenu").SetUsage("use dmennu to select from all configured commands"),
			qrCode(),
		)
	return addOpCommand(cmd, "command",
		func(ctx context.Context, args *opsCmdArgs[[]string]) error {
			cmds, err := getcmds(args.conf.ExportAllCommands(), args.ops)
			if err != nil {
				return err
			}
			return runConfiguredCommand(ctx, cmds)
		})
}

func getcmds(cmds []sardis.CommandConf, args []string) ([]sardis.CommandConf, error) {
	out := make([]sardis.CommandConf, 0, len(args))

	ops := dt.NewSetFromSlice(args)
	collected := dt.Set[string]{}
	collected.Order()

	for idx := range cmds {
		if len(args) == collected.Len() {
			break
		}

		if name := cmds[idx].NamePrime(); ops.Check(name) && ft.Not(collected.Check(name)) {
			out = append(out, cmds[idx])
			collected.Add(name)
		}
	}

	// if we didn't find all that we were looking for?
	if ops.Len() != len(out) {
		return nil, fmt.Errorf("found %d ops, of %d, ops [%s]; found [%s] ",
			len(out), ops.Len(),
			// TODO we should be able to get slices from sets without panic
			strings.Join(fun.NewGenerator(ops.Stream().Slice).Force().Resolve(), ", "),
			strings.Join(fun.NewGenerator(collected.Stream().Slice).Force().Resolve(), ", "),
		)
	}

	return out, nil
}

func runConfiguredCommand(ctx context.Context, cmds dt.Slice[sardis.CommandConf]) error {
	return cmds.Stream().Parallel(func(ctx context.Context, conf sardis.CommandConf) error { return conf.Worker().Run(ctx) },
		fun.WorkerGroupConfContinueOnError(),
		fun.WorkerGroupConfWorkerPerCPU(),
	).Run(ctx)
}

func listCommands() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		Aliases("ls").
		SetUsage("return a list of defined commands").
		With(cmdr.SpecBuilder(ResolveConfiguration).
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				homedir := util.GetHomedir()

				table := tabby.New()
				table.AddHeader("Group", "Name", "Command", "Directory")

				for _, group := range conf.Commands {
					for _, cmd := range group.Commands {
						if cmd.Directory == homedir {
							cmd.Directory = ""
						}

						grps := append([]string{group.Name}, group.Aliases...)
						if group.Name == "run" && !slices.Contains(grps, "*") {
							grps = append(grps, "*")
						}

						nms := strings.Join(append([]string{cmd.Name}, cmd.Aliases...), ", ")
						cmds := append([]string{cmd.Command}, cmd.Commands...)
						for idx := range cmds {
							if maxLen := 52; len(cmds[idx]) > maxLen {
								cmds[idx] = fmt.Sprintf("<%s...>", cmds[idx][:maxLen])
							}
						}
						for idx, chunk := range cmds {
							if idx == 0 {
								table.AddLine(
									strings.Join(grps, ","),                 // group
									nms,                                     // names
									chunk,                                   // command
									sutil.TryCollapseHomedir(cmd.Directory), // dir
								)
							} else {
								table.AddLine(
									"",    // group
									"",    // names
									chunk, // command
									"",    // dir
								)

							}
						}
					}
				}
				table.Print()

				return nil
			}).Add)
}

type bufCloser struct{ bytes.Buffer }

func (b bufCloser) Close() error { return nil }

func qrCode() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("qr").
		SetUsage("gets qrcode from x11 clipboard and renders it on the terminal").
		SetAction(func(ctx context.Context, _ *cli.Context) error {
			buf := &bufCloser{}

			err := jasper.Context(ctx).CreateCommand(ctx).
				AppendArgs("xsel", "--clipboard", "--output").SetOutputWriter(buf).
				Run(ctx)

			if err != nil {
				return fmt.Errorf("problem getting clipboard: %w", err)
			}

			grip.Info(buf.String())
			qrcodeTerminal.New().Get(buf.String()).Print()

			return nil
		})
}
