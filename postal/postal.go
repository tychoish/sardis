package postal

import (
	"github.com/tychoish/fun"
)

type Job interface{ Resolve() fun.Worker }

type Envelope struct {
	Version int
	Type    string
	Format  SerializationFormat
	Payload Message
}

type Message []byte
