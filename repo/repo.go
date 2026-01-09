package repo

import (
	"errors"
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/sardis/subexec"
	"github.com/tychoish/sardis/util"
)

type GitRepository struct {
	Name       string          `bson:"name" json:"name" yaml:"name"`
	Path       string          `bson:"path" json:"path" yaml:"path"`
	Remote     string          `bson:"remote" json:"remote" yaml:"remote"`
	RemoteName string          `bson:"remote_name" json:"remote_name" yaml:"remote_name"`
	Branch     string          `bson:"branch" json:"branch" yaml:"branch"`
	LocalSync  bool            `bson:"sync" json:"sync" yaml:"sync"`
	Fetch      bool            `bson:"fetch" json:"fetch" yaml:"fetch"`
	Notify     bool            `bson:"notify" json:"notify" yaml:"notify"`
	Disabled   bool            `bson:"disabled" json:"disabled" yaml:"disabled"`
	Logs       subexec.Logging `bson:"logs" json:"logs" yaml:"logs"`
	Pre        []string        `bson:"pre" json:"pre" yaml:"pre"`
	Post       []string        `bson:"post" json:"post" yaml:"post"`
	Mirrors    []string        `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
	Tags       []string        `bson:"tags" json:"tags" yaml:"tags"`
}

func (conf *GitRepository) Validate() error {
	if conf.Branch == "" {
		conf.Branch = "main"
	}

	if conf.RemoteName == "" {
		conf.RemoteName = "origin"
	}

	conf.Logs = util.Default(conf.Logs, conf.Logs.Default())
	conf.Path = util.TryExpandHomeDir(conf.Path)
	conf.Post = util.TryExpandHomeDirs(conf.Post)
	conf.Pre = util.TryExpandHomeDirs(conf.Pre)

	ec := &erc.Collector{}
	ec.Push(conf.Logs.Validate())

	if conf.Remote == "" {
		ec.Push(fmt.Errorf("'%s' does not specify a remote", conf.Name))
	}

	if conf.Fetch && conf.LocalSync {
		ec.Push(errors.New("cannot specify sync and fetch"))
	}

	return ec.Resolve()
}

func (conf *GitRepository) HasChanges() (bool, error) {
	if _, err := os.Stat(conf.Path); os.IsNotExist(err) {
		return true, nil
	}

	repo, err := git.PlainOpen(conf.Path)
	if err != nil {
		return false, err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, err
	}

	stat, err := wt.Status()
	if err != nil {
		return false, err
	}

	return !stat.IsClean(), nil
}
