package operations

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
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
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

const commandFlagName string = "command"

func RunCommand() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().SetName("run").
		SetUsage("runs a predefined command").
		Subcommanders(
			listCommands(),
			dmenuCommand(dmenuCommandAll).SetName("dmenu").SetUsage("use dmennu to select from all configured commands"),
			qrCode(),
		),
		commandFlagName, func(ctx context.Context, args *withConf[[]string]) error {
			cmds, err := getcmds(args.conf.Operations.ExportAllCommands(), args.arg)
			if err != nil {
				return err
			}
			return runConfiguredCommand(ctx, cmds)
		})
}

func getcmds(cmds dt.Slice[subexec.Command], args []string) ([]subexec.Command, error) {
	ops := dt.NewSetFromSlice(args)
	seen := dt.Set[string]{}
	seen.Order()

	out := cmds.Filter(func(cmd subexec.Command) bool {
		name := cmd.NamePrime()
		return ops.Check(name) && !seen.AddCheck(name)
	})

	// if we didn't find all that we were looking for?
	if ops.Len() != len(out) {
		return nil, fmt.Errorf("found %d ops, of %d, ops [%s]; found [%s] ",
			len(out), ops.Len(),
			// TODO we should be able to get slices from sets without panic
			strings.Join(fun.NewGenerator(ops.Stream().Slice).Force().Resolve(), ", "),
			strings.Join(fun.NewGenerator(seen.Stream().Slice).Force().Resolve(), ", "),
		)
	}

	return out, nil
}

func toWorkers(st *fun.Stream[subexec.Command]) *fun.Stream[fun.Worker] {
	return fun.MakeConverter(func(conf subexec.Command) fun.Worker { return conf.Worker() }).Stream(st)
}

func runWorkers(ctx context.Context, wf fun.Worker) error { return wf.Run(ctx) }

func runConfiguredCommand(ctx context.Context, cmds dt.Slice[subexec.Command]) error {
	size := cmds.Len()
	switch {
	case size == 1:
		return ft.Ptr(cmds.Index(0)).Worker().Run(ctx)
	case size < runtime.NumCPU():
		return fun.MAKE.WorkerPool(toWorkers(cmds.Stream())).Run(ctx)
	default:
		return cmds.Stream().Parallel(func(ctx context.Context, conf subexec.Command) error { return conf.Worker().Run(ctx) },
			fun.WorkerGroupConfContinueOnError(),
			fun.WorkerGroupConfWorkerPerCPU(),
		).Run(ctx)
	}

}

func listCommands() *cmdr.Commander {
	return cmdr.MakeCommander().
		SetName("list").
		Aliases("ls").
		SetUsage("return a list of defined commands").
		With(StandardSardisOperationSpec().
			SetAction(func(ctx context.Context, conf *sardis.Configuration) error {
				homedir := util.GetHomeDir()

				table := tabby.New()
				table.AddHeader("Name", "Command", "Directory")
				for _, cmd := range conf.Operations.ExportAllCommands() {
					if cmd.Directory == homedir {
						cmd.Directory = ""
					}

					cmds := append([]string{cmd.Command}, cmd.Commands...)

					for idx := range cmds {
						if maxLen := 48; len(cmds[idx]) > maxLen {
							cmds[idx] = fmt.Sprintf("<%s...>", cmds[idx][:maxLen])
						}
					}

					for idx, chunk := range cmds {
						if idx == 0 {
							table.AddLine(
								cmd.Name,                               // name
								chunk,                                  // command
								util.TryCollapseHomeDir(cmd.Directory), // dir
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
