package util

import "github.com/tychoish/fun/ft"

func DropErrorOnDefer(ff func() error) { ft.IgnoreError(ff()) }
