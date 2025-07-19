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
	"github.com/tychoish/sardis"
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
		"name", func(ctx context.Context, args *opsCmdArgs[string]) error {
			conf := args.conf
			name := args.ops
			startAt := time.Now()

			if conf == nil || len(conf.Blog) == 0 {
				return errors.New("no blog configured")
			}

			blog := conf.GetBlog(name)
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
					"host":  conf.Settings.Runtime.Hostname,
				})
				return nil
			}

			grip.Notice(message.BuildPair().
				Pair("op", opName).
				Pair("state", "STARTED").
				Pair("name", name).
				Pair("repo", blog.RepoName).
				Pair("path", repo.Path).
				Pair("dur", time.Since(startAt)),
			)

			defer func() {
				grip.Notice(message.BuildPair().
					Pair("op", opName).
					Pair("state", "COMPLETED").
					Pair("err", err != nil).
					Pair("name", name).
					Pair("repo", blog.RepoName).
					Pair("path", repo.Path).
					Pair("dur", time.Since(startAt)),
				)
			}()

			err = repo.SyncRemoteJob(conf.Settings.Runtime.Hostname).WithErrorFilter(func(err error) error {
				return ers.Wrapf(err, "problem syncing blog %q repo", name)
			}).Join(
				jasper.Context(ctx).CreateCommand(ctx).
					Append(blog.DeployCommands...).
					Directory(repo.Path).
					AddEnv(sardis.EnvVarSardisLogQuietStdOut, "true").
					SetOutputSender(level.Info, grip.Sender()).
					SetErrorSender(level.Error, grip.Sender()).Worker(),
			).Run(ctx)

			if err != nil {
				sardis.RemoteNotify(ctx).Error(message.WrapError(err, message.Fields{
					"op":   opName,
					"repo": repo.Name,
					"path": repo.Path,
					"host": conf.Settings.Runtime.Hostname,
				}))
				return fmt.Errorf("problem deploying the blog %q: %w", name, err)
			}

			grip.Noticef("blog publication complete for %q", name)

			return nil
		})
}
