package units

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
)

func SyncRepo(ctx context.Context, queue amboy.Queue, repo *sardis.RepoConf) error {
	hostname, err := os.Hostname()
	if err != nil {
		return errors.WithStack(err)
	}

	catcher := grip.NewCatcher()
	hasMirrors := false
	wg := &sync.WaitGroup{}
	for _, mirror := range repo.Mirrors {
		if strings.Contains(mirror, hostname) {
			grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
				repo.Path, mirror, hostname)
			continue
		}

		hasMirrors = true
		job := NewRepoSyncRemoteJob(mirror, repo.Path, repo.Pre, nil)
		catcher.Add(queue.Put(ctx, job))
		wg.Add(1)
		go func() {
			defer wg.Done()
			grip.Info(message.Fields{"id": job.ID(), "op": "waiting"})
			amboy.WaitJobInterval(ctx, job, queue, 25*time.Millisecond)
		}()
	}

	if hasMirrors {
		wait := make(chan struct{})
		go func() {
			defer close(wait)
			wg.Wait()
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait:
		}
	}

	if repo.LocalSync {
		catcher.Add(queue.Put(ctx, NewLocalRepoSyncJob(repo.Path, repo.Pre, repo.Post)))
	} else if repo.Fetch {
		catcher.Add(queue.Put(ctx, NewRepoFetchJob(repo)))
	}

	return catcher.Resolve()
}
