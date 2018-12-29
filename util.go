package sardis

import (
	"encoding/json"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

type unmarshaler func([]byte, interface{}) error

func getUnmarshaler(fn string) unmarshaler {
	switch {
	// case strings.HasSuffix(fn, ".bson"):
	// 	return bson.Unmarshal
	case strings.HasSuffix(fn, ".json"):
		return json.Unmarshal
	case strings.HasSuffix(fn, ".yaml"), strings.HasSuffix(fn, ".yml"):
		return yaml.Unmarshal
	default:
		return nil
	}
}
