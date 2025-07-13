package repo

import (
	"errors"
	"fmt"
	"os"

	git "github.com/go-git/go-git/v5"
	"github.com/tychoish/jasper/util"
	sutil "github.com/tychoish/sardis/util"
)

type Configuration struct {
	Name       string   `bson:"name" json:"name" yaml:"name"`
	Path       string   `bson:"path" json:"path" yaml:"path"`
	Remote     string   `bson:"remote" json:"remote" yaml:"remote"`
	RemoteName string   `bson:"remote_name" json:"remote_name" yaml:"remote_name"`
	Branch     string   `bson:"branch" json:"branch" yaml:"branch"`
	LocalSync  bool     `bson:"sync" json:"sync" yaml:"sync"`
	Fetch      bool     `bson:"fetch" json:"fetch" yaml:"fetch"`
	Notify     bool     `bson:"notify" json:"notify" yaml:"notify"`
	Disabled   bool     `bson:"disabled" json:"disabled" yaml:"disabled"`
	Pre        []string `bson:"pre" json:"pre" yaml:"pre"`
	Post       []string `bson:"post" json:"post" yaml:"post"`
	Mirrors    []string `bson:"mirrors" json:"mirrors" yaml:"mirrors"`
	Tags       []string `bson:"tags" json:"tags" yaml:"tags"`
}

func (conf *Configuration) Validate() error {
	if conf.Branch == "" {
		conf.Branch = "main"
	}

	if conf.RemoteName == "" {
		conf.RemoteName = "origin"
	}

	if conf.Remote == "" {
		return fmt.Errorf("'%s' does not specify a remote", conf.Name)
	}

	if conf.Fetch && conf.LocalSync {
		return errors.New("cannot specify sync and fetch")
	}

	conf.Path = util.TryExpandHomedir(conf.Path)
	conf.Post = sutil.TryExpandHomeDirs(conf.Post)
	conf.Pre = sutil.TryExpandHomeDirs(conf.Pre)

	return nil
}

func (conf *Configuration) HasChanges() (bool, error) {
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
