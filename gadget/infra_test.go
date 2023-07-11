package gadget

import (
	"context"
	"testing"
	"time"

	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/x/track"
)

func TestRipgrep(t *testing.T) {
	start := time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jpm := jasper.NewManager(jasper.ManagerOptions{ID: t.Name(), Synchronized: true, MaxProcs: 64, Tracker: ft.Must(track.New(t.Name()))})
	args := RipgrepArgs{
		Types:       []string{"go"},
		Regexp:      "go:generate",
		Path:        "/home/tychoish/neon/cloud",
		Directories: true,
		Unique:      true,
	}
	iter := Ripgrep(ctx, jpm, args)

	count := 0

	for iter.Next(ctx) {
		count++
		grip.Info(iter.Value())
	}
	grip.Error(iter.Close())
	grip.Info(message.Fields{
		"path":  args.Path,
		"count": count,
		"dur":   time.Since(start),
	})
}
