package units

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func NewRepoCloneJob(rconf sardis.RepoConf) fun.Worker {
	const opName = "repo-clone"

	return func(ctx context.Context) error {
		hostname := util.GetHostname()
		startAt := time.Now()

		if _, err := os.Stat(rconf.Path); !os.IsNotExist(err) {
			grip.Info(message.Fields{
				"op":   opName,
				"msg":  "repo exists, skipping clone, running update jobs as needed",
				"path": rconf.Path,
				"repo": rconf.Remote,
				"host": hostname,
			})

			if rconf.LocalSync {
				rconfCopy := rconf
				rconfCopy.Pre = nil
				rconfCopy.Post = nil
				return NewRepoSyncJob(hostname, rconfCopy).Run(ctx)
			}

			if rconf.Fetch {
				return NewRepoFetchJob(rconf).Run(ctx)
			}

			return nil
		}

		sender := send.MakeAnnotating(grip.Sender(), map[string]any{
			"op":   opName,
			"repo": rconf.Name,
			"host": hostname,
		})

		err := jasper.Context(ctx).CreateCommand(ctx).
			ID(fmt.Sprintf("%s.%s.clone", hostname, rconf.Name)).
			Priority(level.Debug).
			Directory(filepath.Dir(rconf.Path)).
			SetOutputSender(level.Info, sender).
			SetErrorSender(level.Warning, sender).
			AppendArgs("git", "clone", rconf.Remote, rconf.Path).
			Append(rconf.Post...).
			Run(ctx)

		msg := message.BuildPair().
			Pair("op", opName).
			Pair("host", hostname).
			Pair("dur", time.Since(startAt)).
			Pair("err", err != nil).
			Pair("repo", rconf.Name).
			Pair("path", rconf.Path).
			Pair("remote", rconf.Remote)

		if err != nil {
			grip.Error(message.WrapError(err, msg))
			return err
		}

		grip.Notice(msg)
		return nil
	}
}
