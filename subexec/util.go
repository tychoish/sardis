package subexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/opt"
	"github.com/tychoish/fun/wpa"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

type Utilities struct{}

var TOOLS Utilities = struct{}{}

func (Utilities) WorkerPool(st iter.Seq[fnx.Worker]) fnx.Worker { return wpa.RunWithPool(st) }
func (Utilities) ToWorker(cmd Command) fnx.Worker               { return cmd.Worker() }
func (Utilities) Converter() fn.Converter[Command, fnx.Worker]  { return TOOLS.ToWorker }
func (Utilities) Handler() fnx.Handler[Command] {
	return func(ctx context.Context, cmd Command) error { return cmd.Worker().Run(ctx) }
}

func (Utilities) CommandPool(st iter.Seq[Command], opts ...opt.Provider[*wpa.WorkerGroupConf]) fnx.Worker {
	return wpa.RunWithPool(irt.Convert(st, TOOLS.ToWorker))
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
		erc.Must(b.buffer.WriteString(m.String()))
		erc.Must(b.buffer.WriteString("\n"))
	}
}
