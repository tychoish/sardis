package operations

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/mattn/go-isatty"
	"github.com/shirou/gopsutil/process"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
)

type withConf[T any] struct {
	conf *sardis.Configuration
	arg  T
}

func addCommandWithConf[T any](
	cmd *cmdr.Commander,
	setup func(*cli.Context) (T, error),
	op func(context.Context, *withConf[T]) error,
) *cmdr.Commander {
	return cmd.With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (*withConf[T], error) {
		conf, err := ResolveConfiguration(ctx, cc)
		if err != nil {
			return nil, err
		}

		arg, err := setup(cc)
		if err != nil {
			return nil, err
		}

		return &withConf[T]{conf: conf, arg: arg}, nil
	}).SetMiddleware(func(ctx context.Context, args *withConf[T]) context.Context {
		return sardis.ContextSetup(
			sardis.WithConfiguration,
			sardis.WithAppLogger,
			sardis.WithJasper,
			sardis.WithRemoteNotify,
		)(ctx, args.conf)
	}).SetAction(op).Add)
}

func addOpCommand[T cmdr.FlagTypes](
	cmd *cmdr.Commander,
	name string,
	op func(ctx context.Context, args *withConf[T]) error,
) *cmdr.Commander {
	var zero T

	return addCommandWithConf(cmd.
		Flags((&cmdr.FlagOptions[T]{}).
			SetName(name).
			SetUsage(fmt.Sprintf("specify one or more %s", name)).
			Flag()),
		func(cc *cli.Context) (T, error) {
			arg := cmdr.GetFlag[T](cc, name)

			if !cc.IsSet(name) {
				switch any(zero).(type) {
				case []string:
					arg = any(append(any(arg).([]string), cc.Args().Slice()...)).(T)
				case string:
					arg = any(cc.Args().First()).(T)
				case int:
					val, err := strconv.ParseInt(cc.Args().First(), 0, 64)
					if err == nil {
						arg = any(int(any(val).(int64))).(T)
					}
				case int64:
					val, err := strconv.ParseInt(cc.Args().First(), 0, 64)
					if err == nil {
						arg = any(val).(T)
					}
				case uint:
					val, err := strconv.ParseUint(cc.Args().First(), 0, 64)
					if err == nil {
						arg = any(uint(any(val).(uint64))).(T)
					}
				case uint64:
					val, err := strconv.ParseUint(cc.Args().First(), 0, 64)
					if err == nil {
						arg = any(val).(T)
					}
				case float64:
					val, err := strconv.ParseFloat(cc.Args().First(), 64)
					if err == nil {
						arg = any(val).(T)
					}
				case []int:
					for _, it := range cc.Args().Slice() {
						val, err := strconv.ParseInt(it, 0, 64)
						if err == nil {
							arg = any(append(any(val).([]int), int(any(arg).(int64)))).(T)
						}
					}
				case []int64:
					for _, it := range cc.Args().Slice() {
						val, err := strconv.ParseInt(it, 0, 64)
						if err == nil {
							arg = any(append(any(val).([]int64), any(val).(int64))).(T)
						}
					}
				}
			}

			return arg, nil
		}, op)
}

type OperationRuntimeInfo struct {
	ShouldBlock bool
	TTY         bool
	ParentPID   int32
	ParentName  string
}

func GetOperationRuntime(ctx context.Context) OperationRuntimeInfo {
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
