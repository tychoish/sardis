package subexec

import (
	"bytes"
	"io"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

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
