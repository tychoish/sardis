package units

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/cdr/amboy"
	"github.com/cdr/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
)

func SyncRepo(ctx context.Context, queue amboy.Queue, repo *sardis.RepoConf) error {
	hostname, err := os.Hostname()
	if err != nil {
		return errors.WithStack(err)
	}

	catcher := grip.NewCatcher()

	for _, mirror := range repo.Mirrors {
		if strings.Contains(mirror, hostname) {
			grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
				repo.Path, mirror, hostname)
			continue
		}
		catcher.Add(queue.Put(ctx, NewRepoSyncRemoteJob(mirror, repo.Path, repo.Pre, nil)))
	}

	// wait here to make sure that the remote job has
	// completed syncing.
	//
	// When we do larger syncing here, we might want to
	// have more dependency system.
	amboy.WaitInterval(ctx, queue, time.Millisecond)

	if repo.LocalSync {
		catcher.Add(queue.Put(ctx, NewLocalRepoSyncJob(repo.Path, repo.Pre, repo.Post)))
	} else if repo.Fetch {
		catcher.Add(queue.Put(ctx, NewRepoFetchJob(repo)))
	}

	return catcher.Resolve()
}
