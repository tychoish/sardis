package execpath

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/testt"
)

func TestFindAllExecutables(t *testing.T) {
	ctx := t.Context()
	execs, err := FindAll(ctx).Slice(ctx)
	testt.Log(t, execs)
	assert.NotError(t, err)
	assert.True(t, false)
}
