package gadget

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

type GoGenerateArgs struct {
	Spec            *BuildOrder
	SearchPath      []string
	ContinueOnError bool
}

func GoGenerate(
	ctx context.Context,
	jpm jasper.Manager,
	args GoGenerateArgs,
) error {
	ec := &erc.Collector{}
	index := args.Spec.Packages.IndexByPackageName()

	out := send.MakeWriter(send.MakePlain())
	out.SetPriority(grip.Sender().Priority())

	opStart := time.Now()
	var numPackages int
	for idx, group := range args.Spec.Order {
		group = fun.Must(itertool.CollectSlice(ctx, fun.Transform(
			itertool.Slice(group),
			func(in string) (string, error) {
				return strings.Replace(index[in].LocalDirectory, args.Spec.Path, "", 1), nil
			},
		)))

		numPackages += len(group)

		cmd := append([]string{"go", "generate"}, group...)

		grip.Debug(message.BuildPair().
			Pair("group", idx).
			Pair("packages", len(group)).
			Pair("cmd", strings.Join(cmd, " ")))

		err := jpm.CreateCommand(ctx).
			ID(fmt.Sprint("generate.", idx)).
			Directory(args.Spec.Path).
			AddEnv("PATH", strings.Join(append(args.SearchPath, "$PATH"), ":")).
			PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
			SetOutputSender(level.Debug, out).
			SetErrorSender(level.Error, out).
			Add(cmd).
			Run(ctx)

		ec.Add(err)

		builder := grip.Build().Level(level.Info)
		msg := builder.PairBuilder().
			Pair("op", "run group command").
			Pair("group", idx+1).
			Pair("items", len(group))

		if err != nil {
			builder.Level(level.Error)
			msg.Pair("err", err)
		}

		builder.Send()

		if !args.ContinueOnError && err != nil {
			break
		}
	}

	grip.Notice(message.BuildPair().
		Pair("op", "go generate").
		Pair("dur", time.Since(opStart)).
		Pair("errors", ec.HasErrors()).
		Pair("groups", len(args.Spec.Order)).
		Pair("packages", numPackages))

	return ec.Resolve()
}
