package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/tychoish/cmdr"
	"github.com/tychoish/fun/srv"
	"github.com/tychoish/grip"
)

type ServiceConfig struct {
	Message string
	Timeout time.Duration
}

func StartService(ctx context.Context, conf *ServiceConfig) error {
	// a simple web server

	counter := &atomic.Int64{}
	web := &http.Server{
		Addr: "127.0.0.1:9001",
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			num := counter.Add(1)

			grip.Infof("got request: %d", num)

			_, _ = rw.Write([]byte(conf.Message))
		}),
	}

	// cleanup functions run as soon as the context is canceled.
	srv.AddCleanup(ctx, func(context.Context) error {
		grip.Info("beginning cleanup")
		return nil
	})

	grip.Infof("starting web service, pid=%d", os.Getpid())

	return srv.GetOrchestrator(ctx).Add(srv.HTTP("hello-world", time.Minute, web))
}

func BuildCommand() *cmdr.Commander {
	// initialize flag with default value
	msgOpt := cmdr.FlagBuilder("hello world").
		SetName("message", "m").
		SetUsage("message returned by handler")

	timeoutOpt := cmdr.FlagBuilder(time.Hour).
		SetName("timeout", "t").
		SetUsage("timeout for service lifecycle")

	// create an operation spec; initialize the builder with the
	// constructor for the configuration type. While you can use
	// the commander directly and have more access to the
	// cli.Command for interacting with command line arguments,
	// the Spec model makes it possible to write more easily
	// testable functions, and limit your exposure to the CLI
	operation := cmdr.SpecBuilder(func(ctx context.Context, cc *cli.Command) (*ServiceConfig, error) {
		return &ServiceConfig{Message: cc.String("message")}, nil
	}).SetMiddleware(func(ctx context.Context, conf *ServiceConfig) context.Context {
		// create a new context with a timeout
		ctx, cancel := context.WithTimeout(ctx, conf.Timeout)

		// this is maybe not meaningful, but means that we
		// cancel this timeout during shutdown and means that
		// we cancel this context during shut down and
		// therefore cannot leak it.
		srv.AddCleanup(ctx, func(context.Context) error { cancel(); return nil })

		// this context is passed to all subsequent options.
		return ctx
	}).SetAction(StartService)

	// build a commander. The root Commander adds service
	// orchestration to the context and manages the lifecylce of
	// services started by commands.
	cmd := cmdr.MakeRootCommander()

	// this that the service will wait for the srv.Orchestrator's
	// services to return rather than canceling the context when
	// the action runs.
	cmd.SetBlocking(true)

	// add flags to Commander
	cmd.Flags(msgOpt.Flag(), timeoutOpt.Flag())

	// add operation to Commander
	cmdr.AddOperationSpec(cmd, operation)

	// return the operation
	return cmd
}

func main() {
	// because the build command is blocking this context means
	// that we'll catch and handle the sig term correctly.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	// run the command
	cmdr.Main(ctx, BuildCommand())
}
