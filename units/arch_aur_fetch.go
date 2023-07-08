package units

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tychoish/fun"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis"
)

func NewArchFetchAurJob(name string, update bool) fun.Worker {
	return func(ctx context.Context) error {
		if name == "" {
			return errors.New("name is not specified")
		}

		conf := sardis.AppConfiguration(ctx)

		dir := filepath.Join(conf.System.Arch.BuildPath, name)

		args := []string{}

		if stat, err := os.Stat(dir); os.IsNotExist(err) {
			args = append(args, "git", "clone", fmt.Sprintf("https://aur.archlinux.org/%s.git", name))
			dir = filepath.Dir(dir)
		} else if !stat.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", dir)
		} else if update {
			args = append(args, "git", "pull", "origin", "master")
		} else {
			grip.Infof("fetch package for '%s' is a noop", name)
			return nil
		}

		return jasper.Context(ctx).
			CreateCommand(ctx).
			Directory(dir).
			Priority(level.Info).
			SetOutputSender(level.Debug, grip.Sender()).
			AppendArgs(args...).
			Run(ctx)
	}
}
