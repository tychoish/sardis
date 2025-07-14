package units

import (
	"context"
	"os"
	"path/filepath"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
	"github.com/tychoish/sardis"
)

func NewSymlinkCreateJob(conf sardis.LinkConf) fun.Worker {
	return func(ctx context.Context) (err error) {
		ec := &erc.Collector{}
		defer func() { err = ec.Resolve() }()

		dst := filepath.Join(conf.Path, conf.Name)

		if _, err = os.Stat(conf.Target); os.IsNotExist(err) {
			grip.Notice(message.Fields{
				"message": "missing target",
				"name":    conf.Name,
				"target":  conf.Target,
			})
			return
		}

		jpm := jasper.Context(ctx)

		if util.FileExists(conf.Path) {
			if !conf.Update {
				return
			}

			var target string
			target, err = filepath.EvalSymlinks(conf.Path)
			if err != nil {
				ec.Add(err)
				return
			}

			if target != conf.Target {
				if conf.RequireSudo {
					ec.Add(jpm.CreateCommand(ctx).Sudo(true).
						SetCombinedSender(level.Info, grip.Sender()).
						AppendArgs("rm", dst).Run(ctx))
				} else {
					ec.Add(os.Remove(dst))
				}

				grip.Info(message.Fields{
					"op":         "removed incorrect link target",
					"old_target": target,
					"name":       conf.Name,
					"target":     conf.Target,
					"ok":         ec.Ok(),
				})
			} else {
				return
			}

		}

		linkDir := filepath.Dir(conf.Target)
		if conf.RequireSudo {
			cmd := jpm.CreateCommand(ctx).Sudo(true).
				SetCombinedSender(level.Info, grip.Sender())

			if _, err := os.Stat(linkDir); os.IsNotExist(err) {
				cmd.AppendArgs("mkdir", "-p", linkDir)
			}

			cmd.AppendArgs("ln", "-s", conf.Target, conf.Path)

			ec.Add(cmd.Run(ctx))

		} else {
			if _, err := os.Stat(linkDir); os.IsNotExist(err) {
				ec.Add(os.MkdirAll(linkDir, 0755))
			}

			ec.Add(os.Symlink(conf.Target, conf.Path))
		}

		grip.Info(message.Fields{
			"op":  "created new symbolic link",
			"src": conf.Path,
			"dst": conf.Target,
			"ok":  ec.Ok(),
		})
		return
	}
}
