// Package depgraph scans all packages in a Go module and produces a
// map of each package's direct intra-module dependencies. It drives
// go/types through golang.org/x/tools/go/packages so that import
// paths are resolved by the same type-checker used during compilation,
// giving accurate canonical paths even in the presence of replace
// directives or vendor trees.
package depgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"golang.org/x/tools/go/packages"
)

// DepGraph maps each package import path to a sorted, deduplicated
// list of its direct dependencies that belong to the same module.
// It marshals to the JSON shape {"<package>": ["<dep>", ...]}.
type DepGraph map[string][]string

// JSON returns the dependency graph encoded as JSON.
func (g DepGraph) JSON() ([]byte, error) { return json.Marshal(g) }

// Packages returns the import paths of all packages in the graph,
// sorted lexicographically.
func (g DepGraph) Packages() []string {
	out := make([]string, 0, len(g))
	for k := range g {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// loadMode is the minimal set of facts needed to build the graph.
// NeedTypes causes go/packages to run the type-checker, which
// populates pkg.Types (*go/types.Package) for every successfully
// loaded package and each of its imports.
const loadMode = packages.NeedName |
	packages.NeedImports |
	packages.NeedModule |
	packages.NeedTypes

// Scan loads every package reachable via "./..." from dir (normally
// the module root) and returns the intra-module dependency graph.
// Only edges whose target shares the same module path as the source
// are included; standard-library and third-party imports are omitted.
//
// Packages that fail to type-check are still recorded in the graph
// with whatever imports could be resolved; a non-nil error is returned
// alongside the partial graph if any package reported errors.
func Scan(dir string) (DepGraph, error) {
	cfg := &packages.Config{
		Mode: loadMode,
		Dir:  dir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("depgraph: loading packages: %w", err)
	}

	modulePath := firstModulePath(pkgs)

	var ec erc.Collector
	graph := make(DepGraph, len(pkgs))

	for _, pkg := range pkgs {
		// Collect any type-checker errors but continue building the graph.
		ec.From(irt.Convert(irt.Slice(pkg.Errors), func(p packages.Error) error { return p }))

		// Only include packages that belong to the target module.
		if modulePath == "" || strings.HasPrefix(pkg.PkgPath, modulePath) {
			graph[pkg.PkgPath] = intraModuleDeps(pkg, modulePath)
		}
	}
	if !ec.Ok() {
		return nil, ec.Resolve()
	}

	return graph, nil
}

func pkgPath(imp *packages.Package) string {
	// imp.Types is a *go/types.Package populated by the type-checker
	// (NeedTypes load mode). Use its Path() for the canonical import
	// path. Fall back to PkgPath when type-checking was skipped or
	if imp.Types != nil {
		return imp.Types.Path()
	}
	return imp.PkgPath
}

// intraModuleDeps returns the sorted import paths of pkg's direct
// imports that belong to modulePath. It uses the *types.Package
// attached to each import to obtain the canonical path as resolved
// by the type-checker rather than the raw string from the source file.
func intraModuleDeps(pkg *packages.Package, modulePath string) []string {
	deps := make([]string, 0, len(pkg.Imports))

	for path := range irt.Unique(irt.Convert(maps.Values(pkg.Imports), pkgPath)) {
		if modulePath == "" || strings.HasPrefix(pkg.PkgPath, modulePath) {
			deps = append(deps, path)
		}
	}

	sort.Strings(deps)
	return deps
}

// firstModulePath returns the module path reported by the first
// package that carries module metadata.
func firstModulePath(pkgs []*packages.Package) string {
	for _, pkg := range pkgs {
		if pkg.Module != nil {
			return pkg.Module.Path
		}
	}
	return ""
}
