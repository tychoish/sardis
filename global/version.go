/*
Package global holds a number of application level constants and
shared configuration resources for the sardis application.
*/
package global

import (
	"time"

	"github.com/tychoish/fun/ft"
)

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var buildRevision = ""

var buildTimeString = ""

func BuildRevision() string { return ft.Default(buildRevision, "<UNKNOWN>") }

func BuildTime() time.Time {
	return ft.IgnoreSecond(time.Parse(time.DateTime, buildTimeString))
}
