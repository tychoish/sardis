package units

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/deciduosity/amboy"
	"github.com/deciduosity/grip"
	"github.com/pkg/errors"
	"github.com/tychoish/sardis"
)

func SyncRepo(ctx context.Context, queue amboy.Queue, conf *sardis.Configuration, name string) error {
	hostname, err := os.Hostname()
	if err != nil {
		return errors.WithStack(err)
	}

	seen := 0
	catcher := grip.NewCatcher()
	for _, repo := range conf.Repo {
		if repo.Name != name {
			continue
		}
		seen++

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
	}

	catcher.NewWhen(seen == 0, "now matching repos found")
	return catcher.Resolve()
}
