package operations

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	fzf "github.com/koki-develop/go-fzf"
	"github.com/mattn/go-isatty"
	"github.com/shirou/gopsutil/process"
	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
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
			stage, err := subexec.WriteCommandList(ctx, &args.conf.Operations, args.arg)
			if err != nil {
				return err
			}

			if stage.Commands != nil {
				return runCommands(ctx, stage.Commands)
			}

			buf := bufio.NewWriter(os.Stdout)
			for _, opt := range stage.Selections {
				if stage.Prefix != "" {
					_, _ = fmt.Fprintf(buf, "%s.%s", stage.Prefix, opt)
					continue
				}
				_, _ = fmt.Fprintln(buf, opt)
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

func operationRuntime(ctx context.Context) OperationRuntimeInfo {
	opr := OperationRuntimeInfo{}

	opr.ParentPID = int32(os.Getppid())
	opr.ShouldBlock = os.Getenv("SARDIS_CMDS_BLOCKING") != ""
	opr.TTY = isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

	parentProc, err := process.NewProcessWithContext(ctx, opr.ParentPID)
	if err != nil {
		grip.Warning(message.Fields{
			"error": err,
			"ppid":  opr.ParentPID,
			"msg":   "falling back to is-a-tty",
			"stage": "gopsuti.NewProcess",
			"tty":   opr.TTY,
		})
		if !opr.ShouldBlock {
			opr.ShouldBlock = opr.TTY
		}
	} else if err == nil {
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

		if !opr.ShouldBlock {
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
		}
		grip.Debug(message.Fields{
			"ppid":   opr.ParentPID,
			"block":  opr.ShouldBlock,
			"parent": opr.ParentName,
			"stage":  "passed",
			"tty":    opr.TTY,
		})
	}
	return opr
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

			opr := operationRuntime(ctx)
			for {
				stage, err := subexec.WriteCommandList(ctx, &args.conf.Operations, op)
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
