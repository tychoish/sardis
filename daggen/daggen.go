package daggen

import (
	"context"

	"github.com/tychoish/grip"
	"golang.org/x/tools/go/packages"
)

func GetDag(ctx context.Context, path string) {
	grip.Info("starting")

	conf := &packages.Config{
		Context: ctx,
		Logf:    grip.Debugf,
		Dir:     path,
	}
	grip.Info(conf)

	files, err := packages.Load(conf, "...")
	grip.Alert(err)
	for _, f := range files {
		grip.Infoln(f.PkgPath, f.Imports)

	}
}
