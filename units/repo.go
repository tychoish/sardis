package units

import (
	"context"
	"os"
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
			if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
				return NewRepoFetchJob(repo).Run(ctx)
			}

			if changes, err := repo.HasChanges(); changes || err != nil {
				return NewRepoSyncJob(hostname, repo).Run(ctx)
			}
		}

		if repo.Fetch || hasMirrors || repo.LocalSync {
			return NewRepoFetchJob(repo).Run(ctx)
		}

		return nil
	}
}
