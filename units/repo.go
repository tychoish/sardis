package units

import (
	"context"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func SyncRepo(repo sardis.RepoConf) fun.Worker {
	hostname := util.GetHostname()
	hasMirrors := false

	workerList, runWorkers := SetupWorkers()

	for _, mirror := range repo.Mirrors {
		if strings.Contains(mirror, hostname) {
			grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
				repo.Path, mirror, hostname)
			continue
		}

		hasMirrors = true
		workerList.PushBack(NewRepoSyncJob(mirror, repo))
	}

	return func(ctx context.Context) error {
		if err := runWorkers(ctx); err != nil {
			return err
		}

		if repo.LocalSync {
			changes, err := repo.HasChanges()

			if changes || err != nil {
				return NewLocalRepoSyncJob(repo)(ctx)
			}

			return NewRepoFetchJob(repo)(ctx)
		}

		if repo.Fetch || hasMirrors {
			return NewRepoFetchJob(repo)(ctx)
		}

		return nil
	}

}
