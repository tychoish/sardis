package operations

import (
	"time"

	"github.com/pkg/errors"
	"github.com/tychoish/amboy"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis"
	"github.com/tychoish/sardis/units"
	"github.com/urfave/cli"
)

func Blog() cli.Command {
	return cli.Command{
		Name:  "blog",
		Usage: "publish/manage blogging",
		Subcommands: []cli.Command{
			blogPublish(),
		},
	}
}

func blogPublish() cli.Command {
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
		Before: mergeBeforeFuncs(requireConfig(), requireStringOrFirstArgSet(blogNameFlag)),
		Action: func(c *cli.Context) error {
			env := sardis.GetEnvironment()
			ctx, cancel := env.Context()
			defer cancel()

			conf := env.Configuration()
			notify := env.Logger()

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
				return errors.Errorf("invalid configuration for '%s'", name)
			}

			if !blog.Enabled {
				grip.Info(message.Fields{
					"op":   "blog publish",
					"repo": repo.Name,
					"msg":  "publication disabled",
				})
				return nil
			}

			if err := amboy.RunJob(ctx, units.NewLocalRepoSyncJob(repo.Path, repo.Pre, repo.Post)); err != nil {
				return errors.Wrap(err, "problem syncing blog repo")
			}

			jpm := env.Jasper()

			startAt := time.Now()
			err := jpm.CreateCommand(ctx).
				AddEnv(sardis.SSHAgentSocketEnvVar, conf.Settings.SSHAgentSocketPath).
				Append(blog.DeployCommands...).
				Directory(repo.Path).
				SetOutputSender(level.Info, grip.GetSender()).
				SetErrorSender(level.Error, grip.GetSender()).
				Run(ctx)
			if err != nil {
				notify.Error(message.WrapError(err, message.Fields{
					"op":   "blog publish",
					"repo": repo.Name,
				}))
				return errors.Wrap(err, "problem running deploy command")
			}

			notify.Notice(message.Fields{
				"op":   "blog publish",
				"repo": repo.Name,
				"dur":  int(time.Since(startAt).Round(time.Second).Seconds()),
			})

			return nil
		},
	}
}
