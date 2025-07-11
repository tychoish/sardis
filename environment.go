/*
Package sardis holds a number of application level constants and
shared configuration resources for the sardis application.
*/
package sardis

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

const ApplicationName = "sardis"

const (
	EnvVarSSHAgentSocket       = "SSH_AUTH_SOCK"
	EnvVarAlacrittySocket      = "ALACRITTY_SOCKET"
	EnvVarSardisLogQuietStdOut = "SARDIS_LOG_QUIET_STDOUT"
	EnvVarSardisLogQuietSyslog = "SARDIS_LOG_QUIET_SYSLOG"
	EnvVarSardisLogFormatJSON  = "SARDIS_LOG_FORMAT_JSON"
	EnvVarSardisLogJSONColor   = "SARDIS_LOG_COLOR_JSON"
)
