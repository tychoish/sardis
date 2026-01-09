package operations

import (
	"context"
	"os"
	"strconv"

	"github.com/mattn/go-isatty"
	"github.com/shirou/gopsutil/process"
	"github.com/urfave/cli/v2"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/subexec"
)

type withConf[T any] struct {
	conf *sardis.Configuration
	arg  T
}

func addOpCommand[T cmdr.FlagTypes](
	cmd *cmdr.Commander,
	name string,
	op func(ctx context.Context, args *withConf[T]) error,
) *cmdr.Commander {
	return cmd.Flags(cmdr.FlagBuilder(false).
		SetName("annotate").
		SetUsage("enable additional annotations").
		Flag(),
	).With(cmdr.SpecBuilder(
		withConfBuilderSpec[T](name),
	).SetMiddleware(func(ctx context.Context, args *withConf[T]) context.Context {
		erc.InvariantOk(args != nil, "must have non-nil args")
		ctx = sardis.WithConfiguration(ctx, args.conf)
		ctx = subexec.WithJasper(ctx, &args.conf.Operations)
		ctx = srv.WithAppLogger(ctx, args.conf.Settings.Logging)
		ctx = srv.WithRemoteNotify(ctx, args.conf.Settings)
		return ctx
	}).SetAction(op).Add)
}

func withConfBuilderSpec[T cmdr.FlagTypes](name string) cmdr.Hook[*withConf[T]] {
	return func(ctx context.Context, cc *cli.Context) (*withConf[T], error) {
		conf, err := ResolveConfiguration(ctx, cc)
		if err != nil {
			return nil, err
		}

		arg, err := embeddedFlag[T](name, cc)
		if err != nil {
			return nil, err
		}

		return &withConf[T]{conf: conf, arg: arg}, nil
	}
}

func embeddedFlag[T cmdr.FlagTypes](name string, cc *cli.Context) (zero T, _ error) {
	arg := cmdr.GetFlag[T](cc, name)

	if !cc.IsSet(name) {
		switch any(zero).(type) {
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
		case string:
			arg = any(cc.Args().First()).(T)
		case []string:
			arg = any(append(any(arg).([]string), cc.Args().Slice()...)).(T)
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
				opr.ShouldBlock = false && !opr.TTY
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
