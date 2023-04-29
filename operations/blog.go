package operations

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Blog(ctx context.Context) cli.Command {
	return cli.Command{
		Name:  "blog",
		Usage: "publish/manage blogging",
		Subcommands: []cli.Command{
			blogPublish(ctx),
		},
	}
}

func blogPublish(ctx context.Context) cli.Command {
	const blogNameFlag = "blog"
	return cli.Command{
		Name:  "publish",
		Usage: "run the publication operation",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  blogNameFlag,
				Usage: "name of the configured blog",
				Value: "blog",
			},
		},
		Before: mergeBeforeFuncs(requireConfig(ctx), requireStringOrFirstArgSet(blogNameFlag)),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment(ctx)
			conf := env.Configuration()

			ctx = sardis.WithRemoteNotify(ctx, conf)
			notify := sardis.RemoteNotify(ctx)

			if conf == nil || len(conf.Blog) == 0 {
				return errors.New("no blog configured")
			}

			name := c.String(blogNameFlag)

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

			if err := amboy.RunJob(ctx, units.NewLocalRepoSyncJob(repo.Path, repo.Branch, repo.Pre, repo.Post)); err != nil {
				return fmt.Errorf("problem syncing blog repo: %w", err)
			}

			jpm := env.Jasper()

			err := jpm.CreateCommand(ctx).
				AddEnv(sardis.SSHAgentSocketEnvVar, conf.SSHAgentSocket()).
				Append(blog.DeployCommands...).
				Directory(repo.Path).
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
		},
	}
}
