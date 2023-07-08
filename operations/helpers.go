package operations

import (
	"context"
	"fmt"
	"strconv"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/sardis"
	"github.com/urfave/cli/v2"
)

type opsCmdArgs[T cmdr.FlagTypes] struct {
	conf *sardis.Configuration
	ops  T
}

func addOpCommand[T cmdr.FlagTypes](
	cmd *cmdr.Commander,
	name string,
	op func(ctx context.Context, args *opsCmdArgs[T]) error,
) *cmdr.Commander {
	return cmd.Flags((&cmdr.FlagOptions[T]{}).
		SetName(name).
		SetUsage(fmt.Sprintf("specify one or more %s", name)).
		Flag(),
	).With(cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Context) (*opsCmdArgs[T], error) {
		conf, err := ResolveConfiguration(ctx, cc)
		if err != nil {
			return nil, err
		}
		out := &opsCmdArgs[T]{conf: conf}
		ops := cmdr.GetFlag[T](cc, name)
		var zero T

		if !cc.IsSet(name) {
			switch any(zero).(type) {
			case []string:
				ops = any(append(any(ops).([]string), cc.Args().Slice()...)).(T)
			case string:
				ops = any(cc.Args().First()).(T)
			case int:
				val, err := strconv.ParseInt(cc.Args().First(), 0, 64)
				if err == nil {
					ops = any(int(any(val).(int64))).(T)
				}
			case int64:
				val, err := strconv.ParseInt(cc.Args().First(), 0, 64)
				if err == nil {
					ops = any(val).(T)
				}
			case uint:
				val, err := strconv.ParseUint(cc.Args().First(), 0, 64)
				if err == nil {
					ops = any(uint(any(val).(uint64))).(T)
				}
			case uint64:
				val, err := strconv.ParseUint(cc.Args().First(), 0, 64)
				if err == nil {
					ops = any(val).(T)
				}
			case float64:
				val, err := strconv.ParseFloat(cc.Args().First(), 64)
				if err == nil {
					ops = any(val).(T)
				}
			case []int:
				for _, it := range cc.Args().Slice() {
					val, err := strconv.ParseInt(it, 0, 64)
					if err == nil {
						ops = any(append(any(ops).([]int), int(any(val).(int64)))).(T)
					}
				}
			case []int64:
				for _, it := range cc.Args().Slice() {
					val, err := strconv.ParseInt(it, 0, 64)
					if err == nil {
						ops = any(append(any(ops).([]int64), any(val).(int64))).(T)
					}
				}
			}
		}

		out.ops = ops

		return out, nil
	}).SetMiddleware(func(ctx context.Context, args *opsCmdArgs[T]) context.Context {
		return sardis.WithRemoteNotify(ctx, args.conf)
	}).SetAction(op).Add)
}
