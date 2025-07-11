package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
)

func Blog() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("blog").
		SetUsage("publish/manage blogging").
		Subcommanders(
			blogPublish(),
		)
}

func blogPublish() *cmdr.Commander {
	const blogNameFlag = "blog"
	return cmdr.MakeCommander().
		SetName("publish").
		SetUsage("run the publication operation").
		Flags(cmdr.FlagBuilder("blog").
			SetName(blogNameFlag).
			SetUsage("name of the configured blog").
			Flag()).
		With(StringSpecBuilder(blogNameFlag, nil).
			SetAction(func(ctx context.Context, name string) error {
				conf := sardis.AppConfiguration(ctx)

				ctx = sardis.WithRemoteNotify(ctx, conf)
				notify := sardis.RemoteNotify(ctx)

				if conf == nil || len(conf.Blog) == 0 {
					return errors.New("no blog configured")
				}

				var repo *sardis.RepoConf
				var blog *sardis.BlogConf
				for idx := range conf.Blog {
					b := conf.Blog[idx]
					if b.RepoName != name {
						continue
					}
					for ridx := range conf.Repo {
						r := conf.Repo[ridx]
						if b.RepoName != r.Name {
							continue
						}
						repo = &r
						blog = &b
					}
				}

				if repo == nil || blog == nil {
					return fmt.Errorf("invalid configuration for '%s'", name)
				}

				if !blog.Enabled {
					grip.Info(message.Fields{
						"op":   "blog publish",
						"repo": repo.Name,
						"msg":  "publication disabled",
					})
					return nil
				}

				if err := units.NewLocalRepoSyncJob(*repo)(ctx); err != nil {
					return fmt.Errorf("problem syncing blog repo: %w", err)
				}

				err := jasper.Context(ctx).CreateCommand(ctx).
					AddEnv(sardis.EnvVarSSHAgentSocket, conf.SSHAgentSocket()).
					Append(blog.DeployCommands...).
					Directory(repo.Path).
					AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
					SetOutputSender(level.Info, grip.Sender()).
					SetErrorSender(level.Error, grip.Sender()).
					Run(ctx)
				if err != nil {
					notify.Error(message.WrapError(err, message.Fields{
						"op":   "blog publish",
						"repo": repo.Name,
					}))
					return fmt.Errorf("problem running deploy command: %w", err)
				}

				return nil
			}).Add)
}
