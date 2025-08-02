package subexec

import (
	"context"

	"github.com/tychoish/fun/srv"
	"github.com/tychoish/jasper"
	"github.com/tychoish/sardis/global"
)

func WithJasper(ctx context.Context, conf *Configuration) context.Context {
	jpm := jasper.NewManager(
		jasper.ManagerOptionSetSynchronized(),
		jasper.ManagerOptionWithEnvVar(global.EnvVarAlacrittySocket, conf.AlacrittySocket()),
		jasper.ManagerOptionWithEnvVar(global.EnvVarSSHAgentSocket, conf.SSHAgentSocket()),
	)
	srv.AddCleanup(ctx, jpm.Close)

	noStdOut := jasper.NewManager(
		jasper.ManagerOptionSetSynchronized(),
		jasper.ManagerOptionWithEnvVar(global.EnvVarAlacrittySocket, conf.AlacrittySocket()),
		jasper.ManagerOptionWithEnvVar(global.EnvVarSSHAgentSocket, conf.SSHAgentSocket()),
		jasper.ManagerOptionWithEnvVar(global.EnvVarSardisLogQuietStdOut, "true"),
	)
	srv.AddCleanup(ctx, noStdOut.Close)

	jasper.WithContextManager(ctx, "without-std-out", noStdOut)
	return jasper.WithManager(ctx, jpm)
}
