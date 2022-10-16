package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Unmarshaler func([]byte, interface{}) error

func GetUnmarshaler(fn string) Unmarshaler {
	switch {
	// case strings.HasSuffix(fn, ".bson"):
	//	return bson.Unmarshal
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
		return errors.Wrapf(err, "problem opening file %s to unmarshal", fn)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrapf(err, "problem reading data from %s", fn)
	}

	return errors.Wrapf(unmarshal(data, out), "problem reading data from '%s'", fn)
}
