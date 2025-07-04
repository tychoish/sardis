package units

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func NewRepoCloneJob(rconf sardis.RepoConf) fun.Worker {
	return func(ctx context.Context) (err error) {
		ec := &erc.Collector{}
		defer func() { err = ec.Resolve() }()

		if _, err := os.Stat(rconf.Path); !os.IsNotExist(err) {
			if rconf.LocalSync {
				rconfCopy := rconf
				rconfCopy.Pre = nil
				rconfCopy.Post = nil
				ec.Add(NewLocalRepoSyncJob(rconfCopy)(ctx))
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

		sender := send.MakeAnnotating(grip.Sender(), map[string]any{
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
			"path": rconf.Path,
			"repo": rconf.Remote,
			"ok":   ec.Ok(),
			"op":   "repo clone",
		})

		return
	}
}
