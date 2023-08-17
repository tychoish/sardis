package postal

import (
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
)

type Job interface{ Resolve() fun.Worker }

type Message []byte

type Schema struct {
	Version int
	Type    string
	Format  SerializationFormat
}

func (s Schema) Validate() error { return nil }

type Envelope struct {
	Schema  Schema  `db:"schema" bson:"schema" json:"schema" yaml:"schema"`
	Payload Message `db:"payload" bson:"payload" json:"payload" yaml:"payload"`

	// internal accounting
	cache    Job
	registry *Registry
	lazy     adt.Once[error]
}

func (e *Envelope) Job() Job           { return e.cache }
func (e *Envelope) Worker() fun.Worker { return e.cache.Resolve() }

func (e *Envelope) MarshalText() ([]byte, error) {
	return e.marshal(SerializationRepresentationText)
}
func (e *Envelope) MarshalBinary() ([]byte, error) {
	return e.marshal(SerializationRepresentationBinary)
}
func (e *Envelope) UnmarshalText(in []byte) error {
	return e.unmarshal(in, SerializationRepresentationText)
}
func (e *Envelope) UnmarshalBinary(in []byte) error {
	return e.unmarshal(in, SerializationRepresentationBinary)
}

func (e *Envelope) marshal(repr SerializationRepresentation) ([]byte, error) {
	codec, ok := e.registry.Codecs[e.Schema.Format]
	if !ok {
		return nil, fmt.Errorf("unregistered serialization %s", e.Schema.Format)
	}

	if codec.Representation != repr {
		return nil, fmt.Errorf("invalid encoding attempt")
	}

	return codec.Encode(e)
}

func (e *Envelope) unmarshal(in []byte, repr SerializationRepresentation) error {
	e.Payload = in

	e.lazy.Do(func() error { return nil }) // override and fire if needed

	codec, ok := e.registry.Codecs[e.Schema.Format]
	if !ok {
		return fmt.Errorf("unregistered serialization %s", e.Schema.Format)
	}

	if codec.Representation != repr {
		return fmt.Errorf("invalid decoding attempt")
	}

	factory, ok := e.registry.Factories.Load(e.Schema)
	if !ok {
		return fmt.Errorf("unregistered serialization %s", e.Schema.Format)
	}

	out := factory()
	if err := codec.Decode(e.Payload, out); err != nil {
		return err
	}

	e.cache = out
	return nil
}

func (e *Envelope) Resolve(r *Registry) (Job, error) {
	e.registry = r

	if err := e.lazy.Resolve(); err != nil {
		return nil, err
	}

	factory, ok := r.Factories.Load(e.Schema)
	if !ok {
		return nil, fmt.Errorf("unregistered type %s", e.Schema.Type)
	}

	out := factory()
	codec, ok := r.Codecs[e.Schema.Format]
	if !ok {
		return nil, fmt.Errorf("unregistered serialization %s", e.Schema.Format)
	}

	if err := codec.Decode(e.Payload, out); err != nil {
		return nil, err
	}

	e.cache = out

	return out, nil
}

type Registry struct {
	Codecs    CodecStore
	Factories dt.Map[Schema, fun.Future[Job]]
}

func MakeJobFactory[T Job](fn fun.Future[T]) fun.Future[Job] { return func() Job { return fn() } }

func (r *Registry) Register(schema Schema, fn fun.Future[Job]) { r.Factories.Add(schema, fn) }

func (r *Registry) MakeEnvelope(s Schema, msg Job) (*Envelope, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	codec, ok := r.Codecs[s.Format]
	if !ok {
		return nil, fmt.Errorf("unregistered serialization %s", s.Format)
	}

	e := &Envelope{
		Schema:   s,
		cache:    msg,
		registry: r,
	}

	e.lazy.Set(func() error {
		mbin, err := codec.Encode(msg)
		if err != nil {
			return err
		}
		e.Payload = mbin
		return nil
	})

	return e, nil
}
