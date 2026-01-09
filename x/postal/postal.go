package postal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/stw"
)

type Job interface{ Resolve() fnx.Worker }

type Registry struct {
	Codecs    CodecStore
	Factories stw.Map[Schema, fn.Future[Job]]
}

func RegisterFactory[T Job](r *Registry, s Schema, fn fn.Future[T]) {
	r.Register(s, func() Job { return fn() })
}

func (r *Registry) Register(schema Schema, fn fn.Future[Job]) { r.Factories.Add(schema, fn) }

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

	e.Lock.Update(LockStatusCreated)

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

type Envelope struct {
	Schema  Schema   `db:"schema" bson:"schema" json:"schema" yaml:"schema"`
	Payload Message  `db:"payload" bson:"payload" json:"payload" yaml:"payload"`
	Lock    LockInfo `db:"lock" bson:"lock" json:"lock" yaml:"lock"`

	// internal accounting
	cache    Job
	registry *Registry
	lazy     adt.Once[error]
}

type Message []byte

func (e *Envelope) Worker() fnx.Worker { return e.cache.Resolve() }

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

func (e *Envelope) MarshalBSON() ([]byte, error) {
	if strings.Contains(string(e.Schema.Format), "bson") {
		return nil, errors.New("incorrect configuration")
	}
	return e.MarshalBinary()
}

func (e *Envelope) UnmarshalBSON(in []byte) error {
	if strings.Contains(string(e.Schema.Format), "bson") {
		return errors.New("incorrect configuration")
	}
	return e.UnmarshalBinary(in)
}

func (e *Envelope) marshal(repr SerializationRepresentation) ([]byte, error) {
	codec, ok := e.registry.Codecs[e.Schema.Format]
	if !ok {
		return nil, fmt.Errorf("unregistered serialization %s", e.Schema.Format)
	}

	if codec.Representation != repr {
		return nil, fmt.Errorf("invalid encoding attempt")
	}

	data, err := codec.Encode(e.cache)
	if err != nil {
		return nil, err
	}

	e.Payload = data

	return codec.Encode(e)
}

func (e *Envelope) unmarshal(in []byte, repr SerializationRepresentation) error {
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

	if err := codec.Decode(in, e); err != nil {
		return err
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

	if e.cache != nil {
		return e.cache, nil
	}

	if e.Payload == nil {
		return nil, errors.New("missing job definition")
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
