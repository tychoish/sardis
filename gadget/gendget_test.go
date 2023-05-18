package gadget

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

func TestGraph(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud")
	assert.NotError(t, err)

	t.Run("NoDuplicatePackages", func(t *testing.T) {
		seen := set.MakeUnordered[string](len(graph.Packages))
		for _, pkg := range graph.Packages {
			seen.Add(pkg.PackageName)
		}
		assert.Equal(t, seen.Len(), len(graph.Packages))
	})
	t.Run("GraphIsComplete", func(t *testing.T) {
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
}
