package units

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
)

func NewRepoCleanupJob(path string) fun.Worker {
	return func(ctx context.Context) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("cannot cleanup %s, no repository exists", path)
		}

		start := time.Now()

		var err error

		defer func() {
			grip.Notice(message.Fields{
				"op":    "repo-cleanup",
				"repo":  path,
				"dur":   time.Since(start),
				"error": err != nil,
			})
		}()

		cmd := jasper.Context(ctx)

		sender := grip.Context(ctx).Sender()

		err = cmd.CreateCommand(ctx).Priority(level.Info).
			Directory(path).
			SetOutputSender(level.Debug, sender).
			SetErrorSender(level.Warning, sender).
			AppendArgs("git", "gc").
			AppendArgs("git", "prune").
			Run(ctx)

		return err
	}
}
