package daggen

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper/util"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
)

// PackageName is unique and should be the key in a map
type PackageInfo struct {
	ModuleName     string   `yaml:"module" json:"module"`             // f.Module.Path   github.com/tychoish/sardis
	PackageName    string   `yaml:"package" json:"package"`           // f.PkgPath       github.com/tychoish/sardis/dgen
	LocalDirectory string   `yaml:"path" json:"path"`                 // f.Module.Dir    /home/tychoish/src/sardis
	Dependencies   []string `yaml:"dependencies" json:"dependencies"` // <computed>      [<[other]PackageName>, <[other]PackageName> ]
}

type Packages []PackageInfo

func (p Packages) IndexByLocalDirectory() map[string]PackageInfo {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.LocalDirectory] = info
	}
	return out
}

func (p Packages) IndexByPackageName() map[string]PackageInfo {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.PackageName] = info
	}
	return out
}

func (p Packages) WriteTo(w io.Writer) (n int64, err error) {
	var size int64
	buf := &bytes.Buffer{}

	enc := yaml.NewEncoder(buf)
	enc.SetIndent(5)
	for idx, v := range p {
		if err := enc.Encode(v); err != nil {
			return size, fmt.Errorf("could not encode %q at %d of %d: %w",
				v.PackageName, idx, len(p), err)
		}
		n, err := buf.WriteTo(w)
		size += n
		if err != nil {
			return size, fmt.Errorf("could not write %q at %d of %d: %w",
				v.PackageName, idx, len(p), err)
		}
	}

	if err := enc.Close(); err != nil {
		return size, err
	}
	return size, nil
}

func Collect(ctx context.Context, path string) (Packages, error) {
	if !strings.HasSuffix(path, "...") {
		path = filepath.Join(path, "...")
	}

	conf := &packages.Config{
		Env: []string{
			"GOPACKAGESDRIVER=off",
			"GO111MODULE=on",
			fmt.Sprint("GOCACHE=", os.Getenv("GOCACHE")),
			fmt.Sprint("HOME=", util.GetHomedir()),
		},
		Context: ctx,
		Logf: grip.NewLogger(send.MakeAnnotating(grip.Sender(),
			message.Fields{"pkg": path, "op": "dag-collect"})).Debugf,
		Dir:  filepath.Dir(path),
		Mode: packages.NeedImports | packages.NeedModule | packages.NeedName | packages.NeedDeps | packages.NeedFiles,
	}

	files, err := packages.Load(conf, path)
	if err != nil {
		return nil, err
	}

	var output Packages
	for _, f := range files {
		if f.Module == nil {
			grip.Warningf("module %q is missing", f)
			continue
		}
		info := PackageInfo{
			PackageName:    f.PkgPath,
			LocalDirectory: filepath.Join(f.Module.Dir, f.PkgPath[len(f.Module.Path):]),
			ModuleName:     f.Module.Path,
		}

		pkgIter := filterLocal(f.Module.Path, f.Imports)
		for pkgIter.Next(ctx) {
			pkg := pkgIter.Value()
			pkgs := set.MakeNewOrdered[string]()

			depPkgIter := filterLocal(f.Module.Path, pkg.Value.Imports)
			for depPkgIter.Next(ctx) {
				dpkg := depPkgIter.Value()
				set.PopulateSet(ctx, pkgs, fun.PairKeys(filterLocal(f.Module.Path, dpkg.Value.Imports)))
			}

			info.Dependencies = fun.Must(itertool.CollectSlice(ctx, pkgs.Iterator()))
			sort.Strings(info.Dependencies)
		}

		output = append(output, info)
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("no packages for %q", path)
	}

	return output, nil
}

func filterLocal(path string, imports map[string]*packages.Package) fun.Iterator[fun.Pair[string, *packages.Package]] {
	return fun.Filter(
		itertool.FromMap(imports),
		func(p fun.Pair[string, *packages.Package]) bool { return strings.HasPrefix(p.Key, path) },
	)
}
