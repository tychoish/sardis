package units

import (
	"context"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func SyncRepo(repo sardis.RepoConf) fun.Worker {
	return func(ctx context.Context) error {
		wg := &fun.WaitGroup{}
		ec := &erc.Collector{}

		hostname := util.GetHostname()

		hasMirrors := false

		for _, mirror := range repo.Mirrors {
			if strings.Contains(mirror, hostname) {
				grip.Infof("skipping mirror %s->%s because it's probably local (%s)",
					repo.Path, mirror, hostname)
				continue
			}

			hasMirrors = true
			wg.Launch(ctx, NewRepoSyncJob(mirror, repo).Operation(ec.Push))
		}

		wg.Worker().Operation(ec.Push).Run(ctx)

		if !ec.Ok() {
			return ec.Resolve()
		}

		if repo.LocalSync {
			if changes, err := repo.HasChanges(); changes || err != nil {
				return NewLocalRepoSyncJob(repo).Run(ctx)
			}

			return NewRepoFetchJob(repo).Run(ctx)
		}

		if repo.Fetch || hasMirrors {
			return NewRepoFetchJob(repo).Run(ctx)
		}

		return nil
	}
}
