package gadget

import (
	"context"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

func TestGraph(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	graph, err := GetBuildOrder(ctx, "/home/tychoish/neon/cloud", "/home/tychoish/neon/cloud/goapp")
	assert.NotError(t, err)

	for idx, group := range graph {
		grip.Info(message.MakeKV(
			message.KV("idx", idx),
			message.KV("size", len(group)),
			message.KV("group", group),
		))
	}
}
