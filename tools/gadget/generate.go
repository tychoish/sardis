package gadget

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/tychoish/fun/erc"
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

	out := send.MakeWriter(send.MakePlain())
	out.SetPriority(grip.Sender().Priority())

	opStart := time.Now()
	var numPackages int
	for idx, group := range args.Spec.Order {
		numPackages += len(group)

		cmd := append([]string{"go", "generate"}, group...)

		grip.Debug(message.BuildKV().
			KV("group", idx).
			KV("packages", len(group)).
			KV("cmd", strings.Join(cmd, " ")))

		err := jpm.CreateCommand(ctx).
			ID(fmt.Sprint("generate.", idx)).
			Directory(args.Spec.Path).
			AddEnv("PATH", strings.Join(append(args.SearchPath, "$PATH"), ":")).
			AddEnv("GOBIN", filepath.Join(args.SearchPath[0], "bin")).
			PreHook(options.NewDefaultLoggingPreHook(level.Debug)).
			SetOutputSender(level.Debug, out).
			SetErrorSender(level.Error, out).
			Add(cmd).
			Run(ctx)

		builder := grip.Build().Level(level.Info)
		msg := builder.
			KV("op", "run group command").
			KV("group", idx+1).
			KV("items", len(group))

		if err != nil {
			builder.Level(level.Error)
			msg.KV("err", err)
			builder.Send()
			if args.ContinueOnError {
				continue
			}
			break
		}

		builder.Send()
	}

	grip.Notice(message.BuildKV().
		KV("op", "go generate").
		KV("dur", time.Since(opStart)).
		KV("ok", ec.Ok()).
		KV("groups", len(args.Spec.Order)).
		KV("packages", numPackages))

	return ec.Resolve()
}
