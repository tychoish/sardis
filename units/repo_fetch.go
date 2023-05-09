package units

import (
	"context"
	"errors"
	"os"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func NewRepoFetchJob(conf sardis.RepoConf) fun.WorkerFunc {
	return erc.WithCollector(func(ctx context.Context, ec *erc.Collector) error {
		if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
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

		cmd := jasper.Context(ctx).CreateCommand(ctx)

		// sender := send.MakeAnnotating(grip.Sender(), map[string]interface{}{
		// 	"repo": conf.Name,
		// })

		cmd.Directory(conf.Path).
			AddEnv(sardis.SSHAgentSocketEnvVar, sardis.AppConfiguration(ctx).SSHAgentSocket())
			// SetOutputSender(level.Info, sender).
			// SetErrorSender(level.Warning, sender)

		if conf.LocalSync && fun.Contains("mail", conf.Tags) {
			cmd.Append(conf.Pre...)
		}

		cmd.AppendArgs("git", "pull", "--keep", "--rebase", "--autostash", conf.RemoteName, conf.Branch)
		cmd.Append(conf.Post...)

		ec.Add(cmd.Run(ctx))

		grip.Notice(message.Fields{
			"path":   conf.Path,
			"repo":   conf.Remote,
			"errors": ec.HasErrors(),
			"op":     "repo fetch",
		})
		return nil
	})
}
