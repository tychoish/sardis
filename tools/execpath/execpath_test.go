package execpath

import (
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestFindAllExecutables(t *testing.T) {
	ctx := t.Context()
	execs := irt.Collect(FindAll(ctx))
	testt.Log(t, execs)
	assert.NotNil(t, execs)
}
