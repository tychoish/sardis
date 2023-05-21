package gadget

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/fun/testt"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/x/track"
)

func TestGraph(t *testing.T) {
	t.Run("DoubleDag", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)

		gone, err := Collect(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)
		gtwo, err := Collect(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)

		buf := &bytes.Buffer{}
		indx := gone.IndexByPackageName()
		for _, pkg := range gtwo {
			other := indx.Get(pkg.PackageName)
			(Packages{pkg, other}).WriteTo(buf)
			testt.Log(t, fmt.Sprintln(), buf.String())
			buf.Reset()
			if !indx.Check(pkg.PackageName) {
				t.Errorf("pkg %q from two not in one", pkg.PackageName)
				continue
			}
			check.Equal(t, pkg.LocalDirectory, other.LocalDirectory)
			check.Equal(t, pkg.ModuleName, other.ModuleName)
			check.EqualItems(t, pkg.Dependencies, other.Dependencies)
		}
	})

	t.Run("NoDuplicatePackages", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)

		graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)

		seen := set.MakeUnordered[string](len(graph.Packages))
		for _, pkg := range graph.Packages {
			seen.Add(pkg.PackageName)
		}
		assert.Equal(t, seen.Len(), len(graph.Packages))
	})
	t.Run("GraphIsComplete", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)

		graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)

		seen := set.MakeUnordered[string](len(graph.Packages))

		totalNodes := 0
		for idx, group := range graph.Order {
			totalNodes += len(group)
			set.PopulateSet(ctx, seen, itertool.Slice(group))
			grip.Info(message.MakeKV(
				message.KV("idx", idx),
				message.KV("size", len(group)),
				message.KV("group", group),
			))
		}
		check.Equal(t, totalNodes, len(graph.Packages))
		check.Equal(t, totalNodes, seen.Len())
	})
	t.Run("GraphIsCorrect", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)

		graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)

		seen := set.MakeUnordered[string](len(graph.Packages))
		pkgs := fun.Mapify(graph.Packages.IndexByPackageName())

		for gidx, group := range graph.Order {
			for eidx, edge := range group {
				pkg := pkgs.Get(edge)
				if len(pkg.Dependencies) == 0 {
					seen.Add(edge)
					continue
				}
				count := 0
				for didx, dep := range pkg.Dependencies {
					count++
					if !seen.Check(dep) {
						grip.Error(message.Fields{
							"gidx": gidx,
							"eidx": eidx,
							"didx": didx,
						})
						ps := Packages{pkg}
						ps.WriteTo(send.MakeWriter(grip.Sender()))
						t.Fatal("missed dependency", edge, "<==", dep, seen.Len(), len(pkgs))
					}
				}
				check.True(t, count > 0)
				seen.Add(edge)
			}
		}

	})

	t.Run("FirstGroupHasNoDependencies", func(t *testing.T) {
		testt.Context(t)
		ctx := testt.Context(t)

		graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
		assert.NotError(t, err)
		assert.True(t, len(graph.Order) >= 1)
		pkgs := fun.Mapify(graph.Packages.IndexByPackageName())
		for _, edge := range graph.Order[0] {
			testt.Log(t, edge)
			check.True(t, pkgs.Check(edge))
			check.Equal(t, 0, len(pkgs.Get(edge).Dependencies))
			if t.Failed() {
				break
			}
		}
	})
	t.Run("OrderingReport", func(t *testing.T) {
		report := func(t *testing.T, graph *BuildOrder) {
			builder := grip.Build().Level(level.Notice)

			msg := builder.PairBuilder().
				Pair("pkgs", len(graph.Packages)).
				Pair("groups", len(graph.Order))

			observed := 0

			var longest int
			for idx, group := range graph.Order {
				observed += len(group)
				grip.Info(message.MakeKV(
					message.KV("idx", idx),
					message.KV("size", len(group)),
					message.KV("group", group),
				))

				if len(group) > longest {
					longest = len(group)
				}
			}
			msg.Pair("longest", longest)
			msg.Pair("observed", observed)
			var numSingle int
			for _, group := range graph.Order {
				if len(group) > 1 {
					numSingle++
				}
				testt.Log(t, group)
				check.NotZero(t, len(group))
			}

			builder.Send()
		}

		t.Run("Full", func(t *testing.T) {
			ctx := testt.Context(t)
			graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
			assert.NotError(t, err)

			report(t, graph)
		})
		t.Run("Narrowed", func(t *testing.T) {
			ctx := testt.Context(t)
			jpm := jasper.NewManager(jasper.ManagerOptions{ID: t.Name(), Synchronized: true, MaxProcs: 64, Tracker: fun.Must(track.New(t.Name()))})

			graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
			assert.NotError(t, err)

			iter := Ripgrep(ctx, jpm, RipgrepArgs{
				Types:       []string{"go"},
				Regexp:      "go:generate",
				Path:        "~/neon/cloud",
				Directories: true,
				Unique:      true,
			})

			limits := set.MakeNewOrdered[string]()
			set.PopulateSet(ctx, limits, graph.Packages.ConvertPathsToPackages(iter))
			report(t, graph.Narrow(limits))
		})
	})
}

func BenchmarkGadget(b *testing.B) {
	for _, p := range fun.MakePairs(
		fun.MakePair("Jasper", "/home/tychoish/src/jasper"),
		fun.MakePair("Grip", "/home/tychoish/src/grip"),
		fun.MakePair("Sardis", "/home/tychoish/src/sardis"),
		fun.MakePair("NeonCloud", "/home/tychoish/neon/cloud"),
	) {
		b.Run(p.Key, func(b *testing.B) {
			b.Run("DagGenCollect", func(b *testing.B) {
				start := time.Now()
				ctx := testt.Context(b)
				var (
					pkgs Packages
					err  error
				)
				for i := 0; i < b.N; i++ {
					pkgs, err = Collect(ctx, p.Value)
					check.NotError(b, err)
					check.True(b, len(pkgs) >= 1)
					b.Log("num-packages", len(pkgs))
				}
				b.Log("duration", time.Since(start))
			})
			b.Run("BuildOrderGenerator", func(b *testing.B) {
				start := time.Now()
				ctx := testt.Context(b)
				var (
					order *BuildOrder
					err   error
				)
				for i := 0; i < b.N; i++ {
					order, err = GetBuildOrder(ctx, p.Value)
					check.NotError(b, err)
					check.True(b, len(order.Packages) >= 1)
					check.True(b, len(order.Order) >= 1)
					b.Log("num-groups", len(order.Order))
				}
				b.Log("duration", time.Since(start))
			})
		})
	}

}
