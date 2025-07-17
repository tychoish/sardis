/*
Package sardis holds a number of application level constants and
shared configuration resources for the sardis application.
*/
package sardis

import "github.com/tychoish/sardis/global"

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

const (
	ApplicationName = global.ApplicationName

	EnvVarSSHAgentSocket       = global.EnvVarSSHAgentSocket
	EnvVarAlacrittySocket      = global.EnvVarAlacrittySocket
	EnvVarSardisLogQuietStdOut = global.EnvVarSardisLogQuietStdOut
	EnvVarSardisLogQuietSyslog = global.EnvVarSardisLogQuietSyslog
	EnvVarSardisLogFormatJSON  = global.EnvVarSardisLogFormatJSON
	EnvVarSardisLogJSONColor   = global.EnvVarSardisLogJSONColor
)
