package daggen

import (
	"context"
	"strings"

	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"golang.org/x/tools/go/packages"
)

func GetDag(ctx context.Context, path string) {
	conf := &packages.Config{
		Context: ctx,
		Logf:    grip.Debugf,
		Dir:     path,
		Mode:    packages.NeedImports | packages.NeedModule | packages.NeedName | packages.NeedDeps | packages.NeedFiles,
	}

	files, err := packages.Load(conf, path)
	grip.Alert(err)

	for _, f := range files {
		lpkg := filterLocal(f.PkgPath, f.Imports)
		grip.Info(message.Fields{"name": f.PkgPath, "packages": len(lpkg)})
		grip.InfoWhen(len(lpkg) != 0, lpkg)
		for name, pkg := range lpkg {
			deps := filterLocal(f.PkgPath, pkg.Imports)
			grip.Notice(message.Fields{"name": name, "packages": len(deps)})
			grip.InfoWhen(len(deps) != 0, deps)
		}
	}
}

func filterLocal(path string, imports map[string]*packages.Package) map[string]*packages.Package {
	out := make(map[string]*packages.Package)
	for k, v := range imports {
		if !strings.HasPrefix(k, path) {
			continue
		}
		out[k] = v
	}
	return out
}
