package util

import (
	"time"

	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

func WithTiming(name string, op func()) {
	start := time.Now()
	defer func() {
		grip.Info(message.BuildPair().
			Pair("op", name).
			Pair("dur", time.Since(start)))
	}()

	op()
}
