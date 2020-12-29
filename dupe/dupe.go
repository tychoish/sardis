package dupe

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cdr/grip"
	"github.com/cdr/grip/message"
)

type DiffMode int

const (
	DiffMissing DiffMode = iota
	DiffSame
	Diff
)

type Options struct {
	Target string
	Mirror string
	Mode   DiffMode
	Header string
}

type state map[string]string

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

		out[path[len(root)+1:]] = strings.Trim(string(content), "\n\t ")

		return nil
	})

	if err != nil {
		return nil, err
	}

	return out, nil
}

type comparsioPayload struct {
	targetName    string
	targetContent string
	targetExists  bool
	mirrorName    string
	mirrorContent string
	mirrorExists  bool
}

// desired cases:
//  find files in the target that are the same as targets in the mirror

func Find(opts Options) ([]string, error) {
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
		mc, ok := mirror[path]
		if ok && mc == tc {
			out = append(out, filepath.Join(opts.Target, path))
		}
	}

	sort.Strings(out)

	grip.Info(message.Fields{
		"target":  opts.Target,
		"mirror":  opts.Mirror,
		"targets": len(target),
		"mirrors": len(mirror),
		"diffs":   len(out),
	})

	return out, nil
}
