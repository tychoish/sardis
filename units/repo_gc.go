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

func NewRepoCleanupJob(path string) fun.WorkerFunc {
	return func(ctx context.Context) error {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("cannot cleanup %s, no repository exists", path)
		}

		grip.Info(message.Fields{
			"job":  "repo-cleanup",
			"path": path,
			"op":   "running",
		})

		cmd := jasper.Context(ctx)

		startAt := time.Now()
		sender := grip.Context(ctx).Sender()

		err := cmd.CreateCommand(ctx).Priority(level.Debug).
			Directory(path).
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Warning, sender).
			AppendArgs("git", "gc").
			AppendArgs("git", "prune").
			Run(ctx)

		grip.Notice(message.Fields{
			"op":    "repo-cleanup",
			"repo":  path,
			"secs":  time.Since(startAt).Seconds(),
			"error": err != nil,
		})

		return err
	}
}
