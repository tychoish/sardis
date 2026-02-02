package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tychoish/birch"
	"github.com/tychoish/fun/erc"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v3"
)

type Unmarshaler func([]byte, interface{}) error

func GetUnmarshaler(fn string) Unmarshaler {
	switch {
	case strings.HasSuffix(fn, ".bson"):
		return bson.Unmarshal
	case strings.HasSuffix(fn, ".json"):
		return json.Unmarshal
	case strings.HasSuffix(fn, ".yaml"), strings.HasSuffix(fn, ".yml"):
		return yaml.Unmarshal
	default:
		return nil
	}
}

func UnmarshalFile(fn string, out interface{}) error {
	if fn == "" {
		return errors.New("file not specified")
	}

	if _, err := os.Stat(fn); os.IsNotExist(err) {
		return fmt.Errorf("file '%s' does not exist", fn)
	}

	unmarshal := GetUnmarshaler(fn)
	if unmarshal == nil {
		return fmt.Errorf("could not find supported unmarshaller for '%s'", fn)
	}

	file, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("problem opening file %s to unmarshal: %w", fn, err)
	}
	defer DropErrorOnDefer(file.Close)

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("problem reading data from %s: %w", fn, err)
	}

	if err := unmarshal(data, out); err != nil {
		return fmt.Errorf("problem reading data from '%s': %w", fn, err)
	}
	return nil
}

type MarshalFormat int

const (
	MarshalFormatJSON MarshalFormat = iota
	MarshalFormatNDJSON
	MarshalFormatBSON
	MarshalFormatYAML
)

func (mf MarshalFormat) String() string {
	switch mf {
	case MarshalFormatJSON:
		return "JSON"
	case MarshalFormatNDJSON:
		return "NDJSON"
	case MarshalFormatBSON:
		return "BSON"
	case MarshalFormatYAML:
		return "YAML"
	default:
		return "UNSPECIFIED"
	}
}

func MarshalerForFile(fn string) MarshalFormat {
	switch filepath.Ext(fn) {
	case "json":
		return MarshalFormatJSON
	case "ndjson", "jsonl":
		return MarshalFormatNDJSON
	case "bson":
		return MarshalFormatBSON
	case "yaml", "yml":
		return MarshalFormatYAML
	default:
		return MarshalFormatYAML
	}
}

func (mf MarshalFormat) GetMarshler() func(any) ([]byte, error) {
	switch mf {
	case MarshalFormatJSON:
		return func(in any) ([]byte, error) { return json.MarshalIndent(in, "", "\t") }
	case MarshalFormatNDJSON:
		return json.Marshal
	case MarshalFormatBSON:
		return func(in any) ([]byte, error) { return birch.DC.Interface(in).MarshalBSON() }
	case MarshalFormatYAML:
		return yaml.Marshal
	}
	panic(erc.NewInvariantError("impossible MarshalFormat value", mf))
}

func (mf MarshalFormat) Marshal(obj any) ([]byte, error) { return mf.GetMarshler()(obj) }

func (mf MarshalFormat) Write(obj any) interface{ Into(io.Writer) error } {
	return newinto(mf.Marshal(obj))
}

func newinto(p []byte, err error) *intoimpl { return &intoimpl{payload: p, err: err} }

type intoimpl struct {
	payload []byte
	err     error
}

func (ii *intoimpl) Into(wr io.Writer) error {
	if ii.err != nil {
		return ii.err
	}
	_, err := wr.Write(ii.payload)
	return err
}
