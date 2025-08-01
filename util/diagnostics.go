package util

import (
	"time"

	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

func LogTiming(name string, op func()) {
	start := time.Now()
	defer func() {
		grip.Info(message.BuildPair().
			Pair("op", name).
			Pair("dur", time.Since(start)))
	}()

	op()
}

func DoWithTiming[T any](op func() T) (val T, itr Interval) {
	defer func() { itr.End = time.Now() }()
	itr.Start = time.Now()

	val = op()

	return
}

func CallWithTiming(op func()) (itr Interval) {
	defer func() { itr.End = time.Now() }()
	itr.Start = time.Now()
	op()

	return
}

type Interval struct {
	Start time.Time
	End   time.Time
}

func (itr Interval) Span() time.Duration {
	return itr.End.Sub(itr.Start)
}
