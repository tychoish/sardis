package units

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/sardis/repo"
)

func NewRepoStatusJob(conf repo.Configuration) fun.Worker { return conf.StatusJob() }
