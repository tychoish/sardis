package units

import (
	"context"
	"errors"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func NewRepoFetchJob(conf sardis.RepoConf) fun.Worker {
	return func(ctx context.Context) (err error) {
		ec := &erc.Collector{}
		defer func() { ec.Add(err); err = ec.Resolve() }()

		start := time.Now()
		defer func() {
			grip.Notice(message.Fields{
				"path":   conf.Path,
				"repo":   conf.Remote,
				"errors": ec.HasErrors(),
				"op":     "repo fetch",
				"dur":    time.Since(start).String(),
			})
		}()

		if !util.FileExists(conf.Path) {
			grip.Info(message.Fields{
				"path": conf.Path,
				"op":   "repo doesn't exist; cloning",
			})

			ec.Add(NewRepoCloneJob(conf)(ctx))
			return nil
		}

		if conf.RemoteName == "" || conf.Branch == "" {
			ec.Add(errors.New("repo-fetch requires defined remote name and branch for the repo"))
			return nil
		}

		// sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		// 	"repo": conf.Name,
		// })

		cmd := jasper.Context(ctx).
			CreateCommand(ctx).
			Directory(conf.Path).
			AddEnv(sardis.SSHAgentSocketEnvVar, sardis.AppConfiguration(ctx).SSHAgentSocket())
			// SetOutputSender(level.Info, sender).
			// SetErrorSender(level.Warning, sender)

		if conf.LocalSync && ft.Contains("mail", conf.Tags) {
			cmd.Append(conf.Pre...)
		}

		cmd.AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", conf.RemoteName, conf.Branch)
		cmd.Append(conf.Post...)

		ec.Add(cmd.Run(ctx))
		return nil
	}
}
