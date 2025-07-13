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
	"github.com/tychoish/sardis/repo"
)

func NewRepoCloneJob(conf repo.Configuration) fun.Worker             { return conf.CloneJob() }
func NewRepoFetchJob(conf repo.Configuration) fun.Worker             { return conf.FetchJob() }
func SyncRepo(conf repo.Configuration) fun.Worker                    { return conf.FullSync() }
func NewLocalRepoSyncJob(conf repo.Configuration) fun.Worker         { return conf.Sync("LOCAL") }
func NewRepoSyncJob(host string, conf repo.Configuration) fun.Worker { return conf.Sync(host) }

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
