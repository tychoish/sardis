package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("problem reading data from %s: %w", fn, err)
	}

	if err := unmarshal(data, out); err != nil {
		return fmt.Errorf("problem reading data from '%s': %w", fn, err)
	}
	return nil
}
