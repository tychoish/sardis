package depgraph_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tychoish/sardis/tools/depgraph"
)

// moduleRoot returns the root of the fun module by walking up from
// this test file's location.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is .../fun/depgraph/depgraph_test.go → parent is module root
	return filepath.Dir(filepath.Dir(file))
}

func TestScan_ReturnsNonEmptyGraph(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(graph) == 0 {
		t.Fatal("expected non-empty dependency graph")
	}
}

func TestScan_PackagesHaveModulePrefix(t *testing.T) {
	const mod = "github.com/tychoish/fun"
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for pkg := range graph {
		if !strings.HasPrefix(pkg, mod) {
			t.Errorf("graph key %q does not start with module path %q", pkg, mod)
		}
	}
}

func TestScan_DepsHaveModulePrefix(t *testing.T) {
	const mod = "github.com/tychoish/fun"
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for pkg, deps := range graph {
		for _, dep := range deps {
			if !strings.HasPrefix(dep, mod) {
				t.Errorf("package %q has non-module dep %q", pkg, dep)
			}
		}
	}
}

func TestScan_DepsAreSorted(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for pkg, deps := range graph {
		for i := 1; i < len(deps); i++ {
			if deps[i] < deps[i-1] {
				t.Errorf("package %q: deps not sorted: %q before %q", pkg, deps[i-1], deps[i])
			}
		}
	}
}

func TestScan_NoDuplicateDeps(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for pkg, deps := range graph {
		seen := make(map[string]bool, len(deps))
		for _, dep := range deps {
			if seen[dep] {
				t.Errorf("package %q: duplicate dep %q", pkg, dep)
			}
			seen[dep] = true
		}
	}
}

func TestScan_NoSelfDeps(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for pkg, deps := range graph {
		for _, dep := range deps {
			if dep == pkg {
				t.Errorf("package %q lists itself as a dependency", pkg)
			}
		}
	}
}

func TestDepGraph_JSON_Shape(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	raw, err := graph.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}

	// Must unmarshal back to the same shape.
	var roundtrip map[string][]string
	if err := json.Unmarshal(raw, &roundtrip); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(roundtrip) != len(graph) {
		t.Errorf("roundtrip len %d != graph len %d", len(roundtrip), len(graph))
	}
	for pkg, deps := range graph {
		rt, ok := roundtrip[pkg]
		if !ok {
			t.Errorf("package %q missing from JSON output", pkg)
			continue
		}
		if len(rt) != len(deps) {
			t.Errorf("package %q: JSON dep count %d != %d", pkg, len(rt), len(deps))
		}
	}
}

func TestDepGraph_Packages_Sorted(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	pkgs := graph.Packages()
	for i := 1; i < len(pkgs); i++ {
		if pkgs[i] < pkgs[i-1] {
			t.Errorf("Packages() not sorted: %q before %q", pkgs[i-1], pkgs[i])
		}
	}
}

func TestDepGraph_Packages_Count(t *testing.T) {
	graph, err := depgraph.Scan(moduleRoot(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got, want := len(graph.Packages()), len(graph); got != want {
		t.Errorf("Packages() len %d != graph len %d", got, want)
	}
}
