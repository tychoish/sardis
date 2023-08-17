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

type Resolver map[SerializationFormat]SerializationCodec

func (r Resolver) Add(codec SerializationCodec) { r[codec.Format] = codec }

func DefaultResolver() Resolver {
	r := Resolver{}
	r.Add(MakeCodecJSON())
	r.Add(MakeCodecDriverBSON())
	r.Add(MakeCodecBirchBSON())
	r.Add(MakeCodecYAML())
	return r
}

type SerializationCodec struct {
	Format SerializationFormat
	Encode func(any) ([]byte, error)
	Decode func([]byte, any) error
}

func MakeCodecJSON() SerializationCodec {
	return SerializationCodec{
		Format: SerializationCodecStdlibJSON,
		Encode: json.Marshal,
		Decode: json.Unmarshal,
	}
}

func MakeCodecYAML() SerializationCodec {
	return SerializationCodec{
		Format: SerializationCodecYAML,
		Encode: yaml.Marshal,
		Decode: yaml.Unmarshal,
	}
}

func MakeCodecDriverBSON() SerializationCodec {
	return SerializationCodec{
		Format: SerializationCodecDriverBSON,
		Encode: bson.Marshal,
		Decode: bson.Unmarshal,
	}
}

func MakeCodecBirchBSON() SerializationCodec {
	return SerializationCodec{
		Format: SerializationCodecDriverBSON,
		Encode: func(in any) ([]byte, error) { return birch.DC.Interface(in).MarshalBSON() },
		Decode: func(in []byte, out any) error { return birch.DC.Reader(in).Unmarshal(out) },
	}
}
