package gadget

import (
	"bytes"
	"context"
	"fmt"
	"go/types"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/stevenle/topsort/v2"
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v3"
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

func (p Packages) IndexByLocalDirectory() map[string]PackageInfo {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.LocalDirectory] = info
	}
	return out
}

func (p Packages) IndexByPackageName() fun.Map[string, PackageInfo] {
	out := make(map[string]PackageInfo, len(p))
	for idx := range p {
		info := p[idx]
		out[info.PackageName] = info
	}
	return out
}

func (p Packages) Graph() fun.Pairs[string, []string] {
	mp := fun.Pairs[string, []string]{}

	for idx := range p {
		mp.Add(p[idx].PackageName, p[idx].Dependencies)
	}

	fun.Invariant(len(p) == len(mp), "graph has impossible structure", len(p), len(mp))

	sort.Slice(mp, func(i, j int) bool { return len(mp[i].Key) > len(mp[j].Key) })

	sort.SliceStable(mp, func(i, j int) bool {
		return len(filepath.SplitList(mp[i].Key)) > len((filepath.SplitList(mp[j].Key)))
	})

	sort.SliceStable(mp, func(i, j int) bool { return len(mp[i].Value) < len(mp[j].Value) })

	return mp
}

func (p Packages) TopsortGraph() *topsort.Graph[string] {
	graph := topsort.NewGraph[string]()
	for _, item := range p.Graph() {
		node := item.Key
		edges := item.Value

		for _, edge := range edges {
			graph.AddEdge(node, edge)
		}
	}
	return graph
}

func (p Packages) WriteTo(w io.Writer) (n int64, err error) {
	var size int64

	buf := bufpool.Get()
	defer bufpool.Put(buf)

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

func (info PackageInfo) String() string {
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	enc := yaml.NewEncoder(buf)
	enc.SetIndent(5)
	fun.InvariantMust(enc.Encode(info))

	return buf.String()
}

func Collect(ctx context.Context, path string) (Packages, error) {
	if !strings.HasSuffix(path, "...") {
		path = filepath.Join(path, "...")
	}

	conf := &packages.Config{
		Context: ctx,
		Logf: grip.NewLogger(send.MakeAnnotating(grip.Sender(),
			message.Fields{"pkg": path, "op": "dag-collect"})).Debugf,
		Dir: filepath.Dir(path),
		// Tests: true,
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

	seen := &fun.Map[string, PackageInfo]{}
	for _, f := range files {
		fun.Invariant(f.Module != nil, "should always collect module information")

		info := PackageInfo{
			PackageName:    f.PkgPath,
			LocalDirectory: filepath.Join(f.Module.Dir, f.PkgPath[len(f.Module.Path):]),
			ModuleName:     f.Module.Path,
		}

		pkgIter := filterLocal(f.Module.Path, f.Types.Imports())
		for pkgIter.Next(ctx) {
			pkg := pkgIter.Value()
			pkgs := set.NewUnordered[string]()

			depPkgIter := filterLocal(f.Module.Path, pkg.Imports())
			for depPkgIter.Next(ctx) {
				dpkg := depPkgIter.Value()
				set.PopulateSet(ctx, pkgs,
					fun.Transform(
						filterLocal(f.Module.Path, dpkg.Imports()),
						func(p *types.Package) (string, error) { return p.Path(), nil },
					),
				)
			}

			info.Dependencies = fun.Must(itertool.CollectSlice(ctx, pkgs.Iterator()))
			sort.Strings(info.Dependencies)
		}
		if seen.Check(info.PackageName) {
			prev := seen.Get(info.PackageName)
			prev.Dependencies = fun.Must(itertool.CollectSlice(ctx, itertool.Uniq(itertool.Slice(append(prev.Dependencies, info.Dependencies...)))))
			info = prev
		}

		seen.Add(info.PackageName, info)
	}

	if seen.Len() == 0 {
		return nil, fmt.Errorf("no packages for %q", path)
	}

	return itertool.CollectSlice(ctx, seen.Values())
}

func filterLocal(path string, imports []*types.Package) fun.Iterator[*types.Package] {
	return fun.Filter(
		itertool.Slice(imports),
		func(p *types.Package) bool {
			return strings.HasPrefix(p.Path(), path)
		})
}
