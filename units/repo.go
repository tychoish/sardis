package units

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
)

func SyncRepo(ctx context.Context, catcher *erc.Collector, wg *sync.WaitGroup, queue amboy.Queue, repo *sardis.RepoConf) {
	hostname, err := os.Hostname()
	if err != nil {
		catcher.Add(err)
		return
	}

	hasMirrors := false
	iwg := &sync.WaitGroup{}
	for _, mirror := range repo.Mirrors {
		if strings.Contains(mirror, hostname) {
			grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
				repo.Path, mirror, hostname)
			continue
		}

		hasMirrors = true
		job := NewRepoSyncJob(mirror, repo.Path, repo.Branch, repo.Pre, nil)
		catcher.Add(queue.Put(ctx, job))
		iwg.Add(1)
		go func() {
			defer iwg.Done()
			amboy.WaitJobInterval(ctx, job, queue, 25*time.Millisecond)
		}()
	}

	startLocal := func() {
		if repo.LocalSync {
			changes, err := repo.HasChanges()
			catcher.Add(err)

			if changes {
				catcher.Add(queue.Put(ctx, NewLocalRepoSyncJob(repo.Path, repo.Branch, repo.Pre, repo.Post)))
			} else {
				catcher.Add(queue.Put(ctx, NewRepoFetchJob(repo)))
			}
		} else if repo.Fetch || hasMirrors {
			catcher.Add(queue.Put(ctx, NewRepoFetchJob(repo)))
		}
	}

	if hasMirrors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iwg.Wait()
			startLocal()
		}()
	} else {
		startLocal()
	}

}
