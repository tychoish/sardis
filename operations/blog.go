package operations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/global"
	"github.com/tychoish/sardis/srv"
	"github.com/tychoish/sardis/util"
)

func Blog() *cmdr.Commander {
	return cmdr.MakeCommander().SetName("blog").
		SetUsage("publish/manage blogging").
		Subcommanders(
			blogPublish(),
		)
}

func blogPublish() *cmdr.Commander {
	const opName string = "blog-publish"
	return addOpCommand(
		cmdr.MakeCommander().SetName("publish").
			SetUsage("run the publication operation"),
		"name", func(ctx context.Context, args *withConf[string]) error {
			conf := args.conf
			name := args.arg
			startAt := time.Now()

			if conf == nil || len(conf.BlogCOMPAT) == 0 {
				return errors.New("no blog configured")
			}

			blog := conf.Repos.ProjectsByName(name)
			if blog == nil {
				return fmt.Errorf("blog %q is not defined", name)
			}

			repo, err := conf.Repos.FindOne(blog.RepoName)
			if err != nil {
				return err
			}
			if repo == nil {
				return fmt.Errorf("repo %q for corresponding blog is not defined", name)
			}

			if !blog.Enabled {
				grip.Warning(message.Fields{
					"op":    opName,
					"state": "disabled",
					"name":  name,
					"repo":  blog.RepoName,
					"path":  repo.Path,
					"host":  util.GetHostname(),
				})
				return nil
			}

			grip.Notice(message.BuildKV().
				KV("op", opName).
				KV("state", "STARTED").
				KV("name", name).
				KV("repo", blog.RepoName).
				KV("path", repo.Path).
				KV("dur", time.Since(startAt)),
			)

			defer func() {
				grip.Notice(message.BuildKV().
					KV("op", opName).
					KV("state", "COMPLETED").
					KV("err", err != nil).
					KV("name", name).
					KV("repo", blog.RepoName).
					KV("path", repo.Path).
					KV("dur", time.Since(startAt)),
				)
			}()

			host := util.GetHostname()
			err = repo.SyncRemoteJob(host).WithErrorFilter(func(err error) error {
				return ers.Wrapf(err, "problem syncing blog %q repo", name)
			}).Join(
				jasper.Context(ctx).CreateCommand(ctx).
					Append(blog.DeployCommands...).
					Directory(repo.Path).
					AddEnv(global.EnvVarSardisLogQuietStdOut, "true").
					SetOutputSender(level.Info, grip.Sender()).
					SetErrorSender(level.Error, grip.Sender()).Worker(),
			).Run(ctx)
			if err != nil {
				srv.RemoteNotify(ctx).Error(message.WrapError(err, message.Fields{
					"op":   opName,
					"repo": repo.Name,
					"path": repo.Path,
					"host": host,
				}))
				return fmt.Errorf("problem deploying the blog %q: %w", name, err)
			}

			grip.Noticef("blog publication complete for %q", name)

			return nil
		})
}
