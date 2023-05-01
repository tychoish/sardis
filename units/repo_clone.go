package units

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/tychoish/amboy"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func NewRepoCloneJob(rconf sardis.RepoConf) fun.WorkerFunc {
	return func(ctx context.Context) (err error) {
		ec := &erc.Collector{}
		defer func() { err = ec.Resolve() }()

		if _, err := os.Stat(rconf.Path); !os.IsNotExist(err) {
			if rconf.LocalSync {
				ec.Add(amboy.RunJob(ctx, NewLocalRepoSyncJob(rconf.Path, rconf.Branch, nil, nil)))
			} else if rconf.Fetch {
				ec.Add(NewRepoFetchJob(rconf)(ctx))
			}

			grip.Notice(message.Fields{
				"path": rconf.Path,
				"repo": rconf.Remote,
				"op":   "exists, skipping clone",
			})
		}

		conf := sardis.AppConfiguration(ctx)
		cmd := jasper.Context(ctx).CreateCommand(ctx)

		sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
			"repo": rconf.Name,
		})

		ec.Add(cmd.ID(strings.Join([]string{rconf.Name, "clone"}, ".")).
			Priority(level.Debug).
			AddEnv(sardis.SSHAgentSocketEnvVar, conf.SSHAgentSocket()).
			Directory(filepath.Dir(rconf.Path)).
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Warning, sender).
			AppendArgs("git", "clone", rconf.Remote, rconf.Path).
			AddWhen(len(rconf.Post) > 0, rconf.Post).
			Run(ctx))

		grip.Notice(message.Fields{
			"path":   rconf.Path,
			"repo":   rconf.Remote,
			"errors": ec.HasErrors(),
			"op":     "repo clone",
		})

		return
	}
}
