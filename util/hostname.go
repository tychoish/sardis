package util

import (
	"os"

	"github.com/tychoish/fun/adt"
)

var hostNameCache *adt.Once[string]

func init() {
	hostNameCache = &adt.Once[string]{}

	hostNameCache.Set(func() string {
		name, err := os.Hostname()
		if err != nil {
			return "UNKNOWN_HOSTNAME"
		}
		return name
	})
}

func GetHostname() string { return hostNameCache.Resolve() }
