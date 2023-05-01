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

func NewArchAbsBuildJob(name string) fun.WorkerFunc {
	return func(ctx context.Context) error {
		if name == "" {
			return errors.New("name is not specified")
		}

		conf := sardis.AppConfiguration(ctx)
		dir := filepath.Join(conf.System.Arch.BuildPath, name)
		pkgbuild := filepath.Join(dir, "PKGBUILD")

		if _, err := os.Stat(pkgbuild); os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist", pkgbuild)
		}

		return jasper.Context(ctx).CreateCommand(ctx).Priority(level.Info).
			AppendArgs("makepkg", "--syncdeps", "--force", "--install", "--noconfirm").
			SetOutputSender(level.Info, grip.Sender()).Directory(dir).Run(ctx)

	}
}
