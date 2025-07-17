package units

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/sardis/repo"
)

func NewRepoCloneJob(conf repo.GitRepository) fun.Worker             { return conf.CloneJob() }
func NewRepoFetchJob(conf repo.GitRepository) fun.Worker             { return conf.FetchJob() }
func SyncRepo(conf repo.GitRepository) fun.Worker                    { return conf.UpdateJob() }
func NewLocalRepoSyncJob(conf repo.GitRepository) fun.Worker         { return conf.SyncRemoteJob("LOCAL") }
func NewRepoSyncJob(host string, conf repo.GitRepository) fun.Worker { return conf.SyncRemoteJob(host) }
func NewRepoCleanupJob(conf repo.GitRepository) fun.Worker           { return conf.CleanupJob() }
func NewRepoStatusJob(conf repo.GitRepository) fun.Worker            { return conf.StatusJob() }
