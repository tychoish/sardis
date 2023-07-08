package units

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
)

func NewArchInstallPackageJob(names []string) fun.Worker {
	return func(ctx context.Context) error {
		args := append([]string{"pacman", "--sync", "--refresh"}, names...)

		return jasper.Context(ctx).CreateCommand(ctx).
			Priority(level.Info).Add(args).
			SetOutputSender(level.Info, grip.Sender()).Run(ctx)
	}
}
