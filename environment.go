/*
Package sardis holds a a number of application level constants and
shared configuration resources for the sardis application.
*/
package sardis

// BuildRevision stores the commit in the git repository at build time
// and is specified with -ldflags at build time
var BuildRevision = ""

const SSHAgentSocketEnvVar = "SSH_AUTH_SOCK"
