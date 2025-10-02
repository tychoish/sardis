package sysmgmt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/util"
)

type LinkConfiguration struct {
	Links []LinkDefinition `bson:"links" json:"links" yaml:"links"`

	ManagedLinkTrees []struct {
		Name string `bson:"name" json:"name" yaml:"name"`
		Sudo bool   `bson:"sudo" json:"sudo" yaml:"sudo"`
	} `bson:"manged" json:"manged" yaml:"manged"`

	Discovery LinkDiscovery `bson:"discovery" json:"discovery" yaml:"discovery"`
	System    struct{}      `bson:"system" json:"system" yaml:"system"`
}

type LinkDefinition struct {
	Name              string `bson:"name" json:"name" yaml:"name"`
	Path              string `bson:"path" json:"path" yaml:"path"`
	Target            string `bson:"target" json:"target" yaml:"target"`
	Update            bool   `bson:"update" json:"update" yaml:"update"`
	DirectoryContents bool   `bson:"directory_contents" json:"directory_contents" yaml:"directory_contents"`
	RequireSudo       bool   `bson:"sudo" json:"sudo" yaml:"sudo"`
}

func (conf *LinkConfiguration) Validate() error {
	ec := &erc.Collector{}
	ec.Push(conf.expand())

	for idx := range conf.Links {
		ec.Wrapf(conf.Links[idx].Validate(), "%d/%d of %T is not valid", idx, len(conf.Links), conf.Links[idx])
	}
	return ec.Resolve()
}

func (lnd *LinkDefinition) Validate() error {
	ec := &erc.Collector{}

	if lnd.Target == "" {
		ec.Push(ers.New("must specify a link target"))
	}

	if lnd.Name == "" {
		fn := filepath.Dir(lnd.Path)

		if fn != "" {
			lnd.Name = fn
		} else {
			ec.Push(ers.New("must specify a name for the link"))
		}
	}

	if lnd.Path == "" {
		base := filepath.Base(lnd.Name)
		fn := filepath.Dir(lnd.Name)
		if base != "" && fn != "" {
			lnd.Path = base
			lnd.Name = fn
		} else {
			ec.Push(ers.New("must specify a location for the link"))
		}
	}

	return ec.Resolve()
}

func (conf *LinkConfiguration) expand() error {
	ec := &erc.Collector{}
	var err error
	hostname := util.GetHostname()
	links := []LinkDefinition{}
	for idx := range conf.Links {
		lnk := conf.Links[idx]

		lnk.Target = strings.ReplaceAll(lnk.Target, "{{hostname}}", hostname)

		if lnk.Target, err = homedir.Expand(lnk.Target); err != nil {
			ec.Add(err)
			continue
		}

		if lnk.Path, err = homedir.Expand(lnk.Path); err != nil {
			ec.Add(err)
			continue
		}

		if lnk.DirectoryContents {
			files, err := os.ReadDir(lnk.Target)
			if err != nil {
				ec.Add(err)
				continue
			}

			for _, info := range files {
				name := info.Name()
				links = append(links, LinkDefinition{
					Name:   fmt.Sprintf("%s:%s", lnk.Name, name),
					Path:   filepath.Join(lnk.Path, name),
					Target: filepath.Join(lnk.Target, name),
					Update: lnk.Update,
				})
			}

			continue
		}

		conf.Links[idx] = lnk
	}
	conf.Links = append(conf.Links, links...)
	return ec.Resolve()
}

func (lnd *LinkDefinition) CreateLinkJob() fun.Worker {
	return func(ctx context.Context) (err error) {
		ec := &erc.Collector{}
		defer func() { err = ec.Resolve() }()

		dst := filepath.Join(lnd.Path, lnd.Name)

		if _, err = os.Stat(lnd.Target); os.IsNotExist(err) {
			grip.Notice(message.Fields{
				"message": "missing target",
				"name":    lnd.Name,
				"target":  lnd.Target,
			})
			return
		}

		jpm := jasper.Context(ctx)

		if _, err = os.Stat(lnd.Path); !os.IsNotExist(err) {
			if !lnd.Update {
				return
			}

			var target string
			target, err = filepath.EvalSymlinks(lnd.Path)
			if err != nil {
				ec.Add(err)
				return
			}

			if target != lnd.Target {
				if lnd.RequireSudo {
					ec.Add(jpm.CreateCommand(ctx).Sudo(true).
						SetCombinedSender(level.Info, grip.Sender()).
						AppendArgs("rm", dst).Run(ctx))
				} else {
					ec.Add(os.Remove(dst))
				}

				grip.Info(message.Fields{
					"op":         "removed incorrect link target",
					"old_target": target,
					"name":       lnd.Name,
					"target":     lnd.Target,
					"ok":         ec.Ok(),
				})
			} else {
				return
			}

		}

		linkDir := filepath.Dir(lnd.Target)
		if lnd.RequireSudo {
			cmd := jpm.CreateCommand(ctx).Sudo(true).
				SetCombinedSender(level.Info, grip.Sender())

			if _, err := os.Stat(linkDir); os.IsNotExist(err) {
				cmd.AppendArgs("mkdir", "-p", linkDir)
			}

			cmd.AppendArgs("ln", "-s", lnd.Target, lnd.Path)

			ec.Add(cmd.Run(ctx))

		} else {
			if _, err := os.Stat(linkDir); os.IsNotExist(err) {
				ec.Add(os.MkdirAll(linkDir, 0o755))
			}

			ec.Add(os.Symlink(lnd.Target, lnd.Path))
		}

		grip.Info(message.Fields{
			"op":  "created new symbolic link",
			"src": lnd.Path,
			"dst": lnd.Target,
			"ok":  ec.Ok(),
		})
		return
	}
}
