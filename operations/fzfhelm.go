package operations

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	fzf "github.com/koki-develop/go-fzf"
	"github.com/mattn/go-isatty"
	"github.com/shirou/gopsutil/process"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

func SearchMenu() *cmdr.Commander {
	return addOpCommand(cmdr.MakeCommander().
		SetName("cmds").
		SetUsage("list or run a command").
		Aliases("c", "m").
		Subcommanders(
			listMenus(),
			fuzzy(),
		),
		"name", func(ctx context.Context, args *withConf[[]string]) error {
			stage, err := WriteCommandList(ctx, &args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if stage.Commands != nil {
				return runCommands(ctx, stage.Commands)
			}

			buf := bufio.NewWriter(os.Stdout)
			for _, opt := range stage.Selections {
				if stage.Prefix != "" {
					fmt.Fprintf(buf, "%s.%s", stage.Prefix, opt)
					continue
				}
				fmt.Fprintln(buf, opt)
			}

			return buf.Flush()
		},
	)
}

type OperationRuntimeInfo struct {
	ShouldBlock bool
	TTY         bool
	ParentPID   int32
	ParentName  string
}

func operationRuntime(ctx context.Context) (context.Context, OperationRuntimeInfo) {
	opr := OperationRuntimeInfo{}

	opr.ParentPID = int32(os.Getppid())

	parentProc, err := process.NewProcessWithContext(ctx, opr.ParentPID)
	opr.TTY = os.Getenv("SARDIS_CMDS_BLOCKING") != "" || (isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()))
	if err != nil {
		grip.Warning(message.Fields{
			"error": err,
			"ppid":  opr.ParentPID,
			"msg":   "falling back to is-a-tty",
			"stage": "gopsuti.NewProcess",
			"tty":   opr.TTY,
		})
		opr.ShouldBlock = opr.TTY
	} else {
		opr.ParentName, err = parentProc.NameWithContext(ctx)
		if err != nil {
			grip.Warning(message.Fields{
				"error": err,
				"ppid":  opr.ParentPID,
				"msg":   "falling back to is-a-tty",
				"stage": "gopsuti.Process.Name",
				"tty":   opr.TTY,
			})
		}

		switch opr.ParentName {
		case "zsh", "bash", "fish", "sh", "nu":
			opr.ShouldBlock = false && ft.Not(opr.TTY)
		case "emacs":
			opr.ShouldBlock = false
		case "ssh":
			opr.ShouldBlock = true
		case "alacritty", "urxvt", "xterm", "Terminal.app":
			opr.ShouldBlock = true
		}

		grip.Debug(message.Fields{
			"ppid":   opr.ParentPID,
			"block":  opr.ShouldBlock,
			"parent": opr.ParentName,
			"stage":  "passed",
			"tty":    opr.TTY,
		})
	}

	var cancel context.CancelFunc
	ctx, cancel = signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	srv.AddCleanup(ctx, fun.MakeOperation(cancel).Worker())
	return ctx, opr
}

func fuzzy() *cmdr.Commander {
	return addOpCommand(
		cmdr.MakeCommander().
			SetName("fuzzy").
			Aliases("fuzz", "fzf", "f", "ff"),
		"name",
		func(ctx context.Context, args *withConf[[]string]) error {
			op := args.arg
			var selected string
			ff, err := fzf.New(
				fzf.WithPrompt(fmt.Sprintf("%s.%s ==> ", util.GetHostname(), sardis.ApplicationName)),
				fzf.WithNoLimit(true),
				fzf.WithCaseSensitive(false),
			)
			if err != nil {
				return err
			}

			ctx, opr := operationRuntime(ctx)
			for {
				stage, err := WriteCommandList(ctx, &args.conf.Operations, op)
				switch {
				case err != nil:
					return err
				case stage.Commands != nil:
					startedAt := time.Now()
					err := runCommands(ctx, stage.Commands)
					ranFor := time.Since(startedAt)

					if opr.ShouldBlock {
						<-ctx.Done()
					}

					waited := time.Since(startedAt) - ranFor

					grip.Notice(message.BuildPair().
						Pair("op", "cmd.fuzzy").
						Pair("state", "COMPLETED").
						Pair("err", err != nil).
						Pair("runtime", ranFor).
						Pair("waited", waited).
						Pair("commands", strings.Join(stage.CommandNames(), ", ")))

					return err
				case stage.Selections != nil:
					idxs, err := ff.Find(
						stage.Selections,
						func(idx int) string {
							return stage.Selections[idx]
						})
					if err != nil {
						return err
					}

					op = make([]string, 0, len(idxs))
					for _, v := range idxs {
						selected = stage.Selections[v]
						if stage.Prefix != "" {
							op = append(op, fmt.Sprintf("%s.%s", stage.Prefix, selected))
						} else {
							op = append(op, selected)
						}
					}
				default:
					// this should be impossible
					return ers.Error("unexpect outcome")
				}
			}
		})
}

type CommandListStage struct {
	NextLabel  string
	Prefix     string
	Selections []string
	Commands   []subexec.Command
}

func (cls CommandListStage) CommandNames() []string {
	if len(cls.Commands) == 0 {
		return nil
	}
	out := make([]string, 0, len(cls.Commands))
	for cmd := range slices.Values(cls.Commands) {
		out = append(out, cmd.FQN())
	}
	return out
}

func WriteCommandList(ctx context.Context, conf *subexec.Configuration, args []string) (*CommandListStage, error) {
	var options []string

	switch len(args) {
	case 0:
		cmds := conf.ExportAllCommands()
		options = make([]string, 0, len(cmds))
		cmds.ReadAll(func(c subexec.Command) {
			options = append(options, c.NamePrime())
		})
		return &CommandListStage{NextLabel: sardis.ApplicationName, Selections: options}, nil
	case 1:
		selection := args[0]
		switch selection {
		case "all", "a":
			return WriteCommandList(ctx, conf, nil)
		case "groups", "group", "g":
			conf.ExportCommandGroups().Keys().ReadAll(func(name string) {
				options = append(options, name)
			}).Run(ctx)
			return &CommandListStage{NextLabel: "groups", Selections: options}, nil
		default:
			groupMap := conf.ExportCommandGroups()

			if gr, ok := groupMap[selection]; ok {
				gr.Commands.ReadAll(func(c subexec.Command) {
					options = append(options, c.NamePrime())
				})
				return &CommandListStage{NextLabel: selection, Selections: options, Prefix: selection}, nil
			}

			cmds, err := getcmds(conf.ExportAllCommands(), args)
			if err != nil {
				return nil, err
			}

			return &CommandListStage{Commands: cmds}, nil
		}
	default:
		switch args[0] {
		case "all", "a", "groups", "group", "g":
			return nil, fmt.Errorf("cannot use keyword %q in context of a multi-command selection %s", args[0], args)
		default:
			groupMap := conf.ExportCommandGroups()

			var missing []string
			var groups []string
			for _, item := range args {
				if _, ok := groupMap[item]; ok {
					groups = append(groups, item)
				} else {
					missing = append(missing, item)
				}
			}

			switch {
			case len(missing) > 0 && len(groups) > 0:
				return nil, fmt.Errorf("ambiguous operation, cannot mix groups %s and commands %s", groups, missing)
			case len(groups) > 0:
				ops := dt.NewSetFromSlice(args)
				if err := groupMap.Keys().Filter(ops.Check).ReadAll(func(name string) {
					options = append(options, name)
				}).Run(ctx); err != nil {
					return nil, err
				}
				return &CommandListStage{NextLabel: "groups", Selections: options}, nil
			case len(missing) > 0:
				cmds, err := getcmds(conf.ExportAllCommands(), args)
				if err != nil {
					return nil, err
				}
				return &CommandListStage{Commands: cmds}, nil
			default:
				panic("unreachable")
			}
		}
	}
}
