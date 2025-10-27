package sysmgmt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/sardis/util"
)

type LinkDiscovery struct {
	// Target paths, if populated, are a list of path-prefixes for the targets of managed links. Use this list to filter
	// links in the search path trees that should be managed. If not specified all links in the search paths are managed.
	TargetPaths []string `bson:"targets" json:"targets" yaml:"targets"`
	// SearchPaths are a a list of paths/trees where the links contained within are (potentially?) managed.
	SearchPaths []string `bson:"search" json:"search" yaml:"search"`
}

func tryAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	return ft.IfElse(ers.IsOk(err), abs, path)
}

func (disco *LinkDiscovery) Validate() error {
	ec := &erc.Collector{}

	if len(disco.SearchPaths) == 0 {
		disco.SearchPaths = append(disco.SearchPaths, util.GetHomeDir())
	}

	disco.SearchPaths = util.TryExpandHomeDirs(util.Apply(tryAbsPath, disco.SearchPaths))
	disco.TargetPaths = util.TryExpandHomeDirs(util.Apply(tryAbsPath, disco.TargetPaths))
	for idx := range disco.SearchPaths {
		path := disco.SearchPaths[idx]
		stat, err := os.Stat(disco.SearchPaths[idx])
		ec.Whenf(os.IsNotExist(err), "link search tree %q does not exist", path)
		ec.Whenf(stat.Mode() != os.ModeDir|os.ModeSymlink,
			"search tree must be either symlinks or directories, %s is %s", path, stat.Mode())
	}
	return ec.Resolve()
}

type PathSearchMap dt.Map[string, PathSearchMap]

func (conf *LinkDiscovery) Resolve() (*dt.Tuples[string, string], error) {
	longest, shortest := 0, 0
	search := PathSearchMap{}
	mapping := search
	for _, path := range conf.TargetPaths {
		parts := strings.Split(path, string(filepath.Separator))
		longest = max(longest, len(parts))
		shortest = min(ft.Default(shortest, len(parts)), len(parts))
		if size := len(parts); size > 0 {
			shortest = max(0)
		}
		for _, elem := range parts {
			if _, ok := mapping[elem]; !ok {
				mapping[elem] = PathSearchMap{}
			}
			mapping = mapping[elem]
		}
		mapping = search
	}

	// TODO: this implementation checks to see if any of the searchpaths are prefixes of the target paths, when what we
	// really want to do is check if any of the target paths are prefixes of the targets of the links in the search paths.
	toTraverse := make([]string, 0, len(conf.SearchPaths))
	for _, tree := range conf.SearchPaths {

		parts := strings.Split(tree, string(filepath.Separator))
		if len(search) == 0 {
			// if there aren't target constraints listed, then we look at everything
			toTraverse = append(toTraverse, tree)
			continue
		}

		if len(parts) > longest {
			continue
		}

		targets := search
		for _, p := range parts {
			if next, ok := targets[p]; ok {
				targets = next
				continue
			} else {
				targets = nil
				break
			}
		}
		if targets != nil {
			toTraverse = append(toTraverse, tree)
		}
	}

	return nil, ers.New("not implemented")
}
