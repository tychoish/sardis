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
				if conf == nil || len(conf.Blog) == 0 {
					return errors.New("no blog configured")
				}

				blog := conf.GetBlog(name)
				if blog == nil {
					return fmt.Errorf("blog %q is not defined", name)
				}

				repo := conf.GetRepo(name)
				if repo == nil {
					return fmt.Errorf("repo %q for corresponding blog is not defined", name)
				}
				if !blog.Enabled {
					grip.Info(message.Fields{
						"op":   "blog publish",
						"repo": repo.Name,
						"msg":  "publication disabled",
					})
					return nil
				}

				if err := units.NewRepoSyncJob(conf.Settings.Runtime.Hostname, *repo)(ctx); err != nil {
					return fmt.Errorf("problem syncing blog repo: %w", err)
				}

				err := jasper.Context(ctx).CreateCommand(ctx).
					Append(blog.DeployCommands...).
					Directory(repo.Path).
					AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
					SetOutputSender(level.Info, grip.Sender()).
					SetErrorSender(level.Error, grip.Sender()).
					Run(ctx)

				if err != nil {
					sardis.RemoteNotify(ctx).Error(message.WrapError(err, message.Fields{
						"op":   "blog-publish",
						"repo": repo.Name,
						"path": repo.Path,
					}))
					return fmt.Errorf("problem running deploy command: %w", err)
				}

				grip.Infof("blog publication complete for %q", name)

				return nil
			}).Add)
}
