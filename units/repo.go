package units

import (
	"context"
	"strings"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/util"
)

func SyncRepo(ctx context.Context, repo sardis.RepoConf) fun.WorkerFunc {
	hostname := util.GetHostname()

	workerList, runWorkers := SetupWorkers()
	hasMirrors := false
	for _, mirror := range repo.Mirrors {
		if strings.Contains(mirror, hostname) {
			grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
				repo.Path, mirror, hostname)
			continue
		}

		hasMirrors = true
		workerList.PushBack(WorkerJob(NewRepoSyncJob(mirror, repo.Path, repo.Branch, repo.Pre, nil)))
	}

	err := runWorkers(ctx)

	return func(ctx context.Context) error {
		if err != nil {
			return err
		}

		if repo.LocalSync {
			changes, err := repo.HasChanges()
			if err != nil {
				return err
			}

			if changes {
				return amboy.RunJob(ctx, NewLocalRepoSyncJob(repo.Path, repo.Branch, repo.Pre, repo.Post))
			}
			return amboy.RunJob(ctx, NewRepoFetchJob(repo))
		}

		if repo.Fetch || hasMirrors {
			return amboy.RunJob(ctx, NewRepoFetchJob(repo))
		}

		return nil
	}

}
