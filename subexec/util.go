package subexec

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

type Utilities struct{}

var TOOLS Utilities = struct{}{}

func (Utilities) Converter() fun.Converter[Command, fun.Worker] { return TOOLS.CommandToWorker }
func (Utilities) CommandToWorker(_ context.Context, c Command) (fun.Worker, error) {
	return c.Worker(), nil
}

func (Utilities) CommandPool(st *fun.Stream[Command]) fun.Worker {
	return TOOLS.WorkerPool(TOOLS.Converter().Stream(st))
}

func (Utilities) WorkerPool(st *fun.Stream[fun.Worker]) fun.Worker {
	return st.Parallel(
		func(ctx context.Context, wf fun.Worker) error { return wf.Run(ctx) },
		fun.WorkerGroupConfContinueOnError(),
		fun.WorkerGroupConfWorkerPerCPU(),
	)
}

type Logging string

const (
	LoggingErrorsOnly Logging = "errors-only"
	LoggingSuppress   Logging = "none"
	LoggingFull       Logging = "full"
)

func (ll Logging) Default() Logging { return LoggingErrorsOnly }
func (ll Logging) Validate() error {
	switch ll {
	case "": // default to errors-only
		return nil
	case LoggingErrorsOnly, LoggingSuppress, LoggingFull:
		return nil
	default:
		return fmt.Errorf("%q is not a valid Logging configuration", ll)
	}
}

func (ll Logging) Full() bool       { return ll == LoggingFull }
func (ll Logging) ErrorsOnly() bool { return ll == LoggingErrorsOnly }
func (ll Logging) Suppress() bool   { return ll == LoggingSuppress }

var bufpool = adt.MakeBytesBufferPool(0)

type OutputBuf struct {
	send.Base
	buffer *bytes.Buffer
}

func NewOutputBuf(id string) (grip.Logger, *OutputBuf) {
	procout := &OutputBuf{buffer: bufpool.Get()}
	procout.SetPriority(level.Info)
	procout.SetName(id)
	procout.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))
	return grip.NewLogger(procout), procout
}

func (b *OutputBuf) Reader() io.Reader { return b.buffer }
func (b *OutputBuf) Writer() io.Writer { return b.buffer }
func (b *OutputBuf) String() string    { return b.buffer.String() }

func (b *OutputBuf) Close() error {
	if b.buffer == nil {
		// make it safe to run more than once
		return nil
	}
	b.buffer.Reset()
	bufpool.Put(b.buffer)
	b.buffer = nil
	return nil
}

func (b *OutputBuf) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString(m.String())))
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString("\n")))
	}
}
