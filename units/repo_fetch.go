package units

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func NewRepoFetchJob(conf sardis.RepoConf) fun.Worker {
	return func(ctx context.Context) (err error) {
		start := time.Now()
		defer func() {
			grip.Info(message.Fields{
				"op":    "repo-fetch",
				"path":  conf.Path,
				"err":   err != nil,
				"repo":  conf.Remote,
				"dur":   time.Since(start).String(),
				"host":  util.GetHostname(),
				"dir":   conf.Path,
				"npre":  len(conf.Pre),
				"npost": len(conf.Post),
			})

		}()

		if !util.FileExists(conf.Path) {
			grip.Info(message.Fields{
				"path": conf.Path,
				"op":   "repo doesn't exist; cloning",
			})

			return NewRepoCloneJob(conf).Run(ctx)
		}

		if conf.RemoteName == "" || conf.Branch == "" {
			return errors.New("repo-fetch requires defined remote name and branch for the repo")
		}

		cmd := jasper.Context(ctx).
			CreateCommand(ctx).
			Directory(conf.Path)
			// SetOutputSender(level.Trace, sender).
			// SetErrorSender(level.Debug, sender)

		if conf.LocalSync && slices.Contains(conf.Tags, "mail") {
			cmd.Append(conf.Pre...)
		}

		cmd.AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", conf.RemoteName, conf.Branch)
		cmd.Append(conf.Post...)

		return cmd.Run(ctx)
	}
}
