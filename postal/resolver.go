package postal

import (
	"encoding/json"

	"github.com/tychoish/birch"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
)

type SerializationFormat string

const (
	SerializationCodecStdlibJSON SerializationFormat = "stdlib-json"
	SerializationCodecBirchBSON  SerializationFormat = "birch-bson"
	SerializationCodecDriverBSON SerializationFormat = "driver-bson"
	SerializationCodecYAML       SerializationFormat = "yaml"
)

type SerializationRepresentation bool

const (
	SerializationRepresentationText   = false
	SerializationRepresentationBinary = true
)

type CodecStore map[SerializationFormat]SerializationCodec

func (r CodecStore) Add(codec SerializationCodec) { r[codec.Format] = codec }

func DefaultCodecStore() CodecStore {
	r := CodecStore{}
	r.Add(MakeCodecJSON())
	r.Add(MakeCodecDriverBSON())
	r.Add(MakeCodecBirchBSON())
	r.Add(MakeCodecYAML())
	return r
}

type SerializationCodec struct {
	Format         SerializationFormat
	Encode         func(any) ([]byte, error)
	Decode         func([]byte, any) error
	Representation SerializationRepresentation
}

func MakeCodecJSON() SerializationCodec {
	return SerializationCodec{
		Format:         SerializationCodecStdlibJSON,
		Encode:         json.Marshal,
		Decode:         json.Unmarshal,
		Representation: SerializationRepresentationText,
	}
}

func MakeCodecYAML() SerializationCodec {
	return SerializationCodec{
		Format:         SerializationCodecYAML,
		Encode:         yaml.Marshal,
		Decode:         yaml.Unmarshal,
		Representation: SerializationRepresentationText,
	}
}

func MakeCodecDriverBSON() SerializationCodec {
	return SerializationCodec{
		Format:         SerializationCodecDriverBSON,
		Encode:         bson.Marshal,
		Decode:         bson.Unmarshal,
		Representation: SerializationRepresentationBinary,
	}
}

func MakeCodecBirchBSON() SerializationCodec {
	return SerializationCodec{
		Format:         SerializationCodecDriverBSON,
		Encode:         func(in any) ([]byte, error) { return birch.DC.Interface(in).MarshalBSON() },
		Decode:         func(in []byte, out any) error { return birch.DC.Reader(in).Unmarshal(out) },
		Representation: SerializationRepresentationBinary,
	}
}
