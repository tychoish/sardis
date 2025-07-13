package repo

import (
	"bytes"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

type bufsend struct {
	send.Base
	buffer bytes.Buffer
}

func (b *bufsend) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString(m.String())))
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString("\n")))
	}
}
