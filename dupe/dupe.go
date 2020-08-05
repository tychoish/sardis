package dupe

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deciduosity/grip"
	"github.com/deciduosity/grip/message"
)

type Semantics int

const (
	NameAndContent Semantics = iota
	NameOnly
)

type Options struct {
	Target    string
	Mirror    string
	Semantics Semantics
}

type state map[string]string

// TODO: add results type

func readTree(root string) (state, error) {
	out := state{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		content, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		out[path] = strings.Trim(string(content), "\n\t ")

		return nil
	})

	if err != nil {
		return nil, err
	}

	return out, nil
}

func ListDiffs(opts Options) ([]string, error) {
	target, err := readTree(opts.Target)
	if err != nil {
		return nil, err
	}

	mirror, err := readTree(opts.Mirror)
	if err != nil {
		return nil, err
	}

	out := []string{}
	for path, tc := range target {
		if mc, ok := mirror[path]; ok {
			if mc == tc {
				continue
			}
		}

		out = append(out, path)
	}

	sort.Strings(out)

	grip.Info(message.Fields{
		"targets": len(target),
		"mirror":  len(mirror),
		"diffs":   len(out),
	})

	return out, nil
}
