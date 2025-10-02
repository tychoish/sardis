// package global is a collection of application-wide references and
// constants that need to be accessible in all packages in the
// application. The package should depend on _no_ other packages
// inside of this module/application.
package global

const ApplicationName = "sardis"

const (
	EnvVarSSHAgentSocket       = "SSH_AUTH_SOCK"
	EnvVarAlacrittySocket      = "ALACRITTY_SOCKET"
	EnvVarSardisLogQuietStdOut = "SARDIS_LOG_QUIET_STDOUT"
	EnvVarSardisLogQuietSyslog = "SARDIS_LOG_QUIET_SYSLOG"
	EnvVarSardisLogFormatJSON  = "SARDIS_LOG_FORMAT_JSON"
	EnvVarSardisLogJSONColor   = "SARDIS_LOG_COLOR_JSON"
	EnvVarSardisAnnotate       = "SARDIS_ANNOTATE_OUTPUT"
)

const (
	ContextDesktopLogger = "desktop"
	ContextRemoteLogger  = "remote"
	ContextTwitterLogger = "twitter"
)

const (
	MenuCommanderDefaultAnnotationSeparator string = "\t"
)
