package gadget

import (
	"bytes"
	"context"
	"fmt"
	"go/types"
	"io"
	"iter"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

var bufpool = &adt.Pool[*bytes.Buffer]{}

func init() {
	bufpool.SetConstructor(func() *bytes.Buffer { return new(bytes.Buffer) })
	bufpool.SetCleanupHook(func(buf *bytes.Buffer) *bytes.Buffer {
		// copy straight from fmt's buffer pool (avoid overly
		// large buffers)
		if buf.Cap() > 64*1024 {
			return nil
		}

		buf.Reset()
		return buf
	})
}

// PackageName is unique and should be the key in a map
type PackageInfo struct {
	ModuleName     string   `yaml:"module" json:"module"`             // f.Module.Path   github.com/tychoish/sardis
	PackageName    string   `yaml:"package" json:"package"`           // f.PkgPath       github.com/tychoish/sardis/dgen
	LocalDirectory string   `yaml:"path" json:"path"`                 // f.Module.Dir    /home/tychoish/src/sardis
	Dependencies   []string `yaml:"dependencies" json:"dependencies"` // <computed>      [<[other]PackageName>, <[other]PackageName> ]
}

type Packages []PackageInfo

type Module struct {
	Path     string
	Packages Packages
}

func (p Packages) IndexByLocalDirectory() map[string]PackageInfo {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.LocalDirectory] = info
	}
	return out
}

func (p Packages) IndexByPackageName() stw.Map[string, PackageInfo] {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.PackageName] = info
	}
	return out
}

func (p Packages) ConvertPathsToPackages(iter iter.Seq[string]) iter.Seq[string] {
	index := p.IndexByLocalDirectory()
	return func(yield func(string) bool) {
		for path := range iter {
			if !yield(index[path].PackageName) {
				return
			}
		}
	}
}

func (p Packages) ConvertPackagesToPaths(iter iter.Seq[string]) iter.Seq[string] {
	index := p.IndexByPackageName()
	return func(yield func(string) bool) {
		for path := range iter {
			if !yield(index[path].LocalDirectory) {
				return
			}
		}
	}
}

func (p Packages) Graph() *dt.List[irt.KV[string, []string]] {
	mp := &dt.List[irt.KV[string, []string]]{}

	for idx := range p {
		mp.PushBack(irt.MakeKV(p[idx].PackageName, p[idx].Dependencies))
	}

	// sort.SliceStable(mp, func(i, j int) bool {
	// 	return len(filepath.SplitList(mp[i].Key)) > len((filepath.SplitList(mp[j].Key)))
	// })

	// sort.SliceStable(mp, func(i, j int) bool { return len(mp[i].Value) < len(mp[j].Value) })

	return mp
}

func (p Packages) WriteTo(w io.Writer) (size int64, err error) {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	enc := yaml.NewEncoder(buf)
	enc.SetIndent(5)
	for idx, v := range p {
		if err = enc.Encode(v); err != nil {
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

func (info PackageInfo) String() string {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	enc := yaml.NewEncoder(buf)
	enc.SetIndent(5)
	erc.Invariant(enc.Encode(info))

	return buf.String()
}

func Collect(ctx context.Context, path string) (*Module, error) {
	out := &Module{
		Path: path,
	}
	if !strings.HasSuffix(path, "...") {
		path = filepath.Join(path, "...")
	}

	conf := &packages.Config{
		Context: ctx,
		Logf: grip.NewLogger(send.MakeAnnotating(grip.Sender(),
			message.Fields{"pkg": path, "op": "dag-collect"})).Debugf,
		Dir:  filepath.Dir(path),
		Mode: packages.NeedModule | packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedTypes,
	}

	// almost all of the time spent is any given operation is in
	// this function. You can cache the contents of the file in a
	// map and pass it to this function, but it doesn't really
	// help much.
	files, err := packages.Load(conf, path)
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool { return files[i].PkgPath < files[j].PkgPath })

	seen := &stw.Map[string, PackageInfo]{}
	for _, f := range files {
		erc.InvariantOk(f.Module != nil, "should always collect module information")

		info := PackageInfo{
			PackageName:    f.PkgPath,
			LocalDirectory: filepath.Join(f.Module.Dir, f.PkgPath[len(f.Module.Path):]),
			ModuleName:     f.Module.Path,
		}

		pkgIter := filterLocal(f.Module.Path, f.Types.Imports())
		for pkg := range pkgIter {
			pkgs := &dt.Set[string]{}

			depPkgIter := filterLocal(f.Module.Path, pkg.Imports())

			for dpkg := range depPkgIter {
				pkgs.Extend(
					irt.Convert(
						filterLocal(f.Module.Path, dpkg.Imports()),
						func(p *types.Package) string { return p.Path() },
					),
				)
			}

			info.Dependencies = irt.Collect(pkgs.Iterator())
			sort.Strings(info.Dependencies)
		}
		if seen.Check(info.PackageName) {
			prev := seen.Get(info.PackageName)
			prev.Dependencies = irt.Collect(irt.Unique(irt.Slice(append(prev.Dependencies, info.Dependencies...))))
			info = prev
		}

		seen.Store(info.PackageName, info)
	}

	if seen.Len() == 0 {
		return nil, fmt.Errorf("no packages for %q", path)
	}

	out.Packages = irt.Collect(seen.Values())

	return out, nil
}

func filterLocal(path string, imports []*types.Package) iter.Seq[*types.Package] {
	return func(yield func(*types.Package) bool) {
		for p := range slices.Values(imports) {
			if strings.HasPrefix(p.Path(), path) {
				if !yield(p) {
					return
				}
			}
		}
	}
}
