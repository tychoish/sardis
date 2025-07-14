package units

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/sardis/repo"
)

func NewRepoCloneJob(conf repo.Configuration) fun.Worker             { return conf.CloneJob() }
func NewRepoFetchJob(conf repo.Configuration) fun.Worker             { return conf.FetchJob() }
func SyncRepo(conf repo.Configuration) fun.Worker                    { return conf.FullSync() }
func NewLocalRepoSyncJob(conf repo.Configuration) fun.Worker         { return conf.Sync("LOCAL") }
func NewRepoSyncJob(host string, conf repo.Configuration) fun.Worker { return conf.Sync(host) }
func NewRepoCleanupJob(conf repo.Configuration) fun.Worker           { return conf.CleanupJob() }
func NewRepoStatusJob(conf repo.Configuration) fun.Worker            { return conf.StatusJob() }
