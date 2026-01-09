package sysmgmt

import (
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
)

type LinkDiscovery struct {
	// SearchPaths are a a list of paths/trees where the links contained within are (potentially?) managed.
	SearchPaths          []string `bson:"search" json:"search" yaml:"search"`
	IgnorePathPrefixes   []string `bson:"ignore_path_prefixes" json:"ignore_path_prefixes" yaml:"ignore_path_prefixes"`
	IgnoreTargetPrefixes []string `bson:"ignore_target_prefixes" json:"ignore_target_prefixes" yaml:"ignore_target_prefixes"`

	SkipMissingTargets  *bool `bson:"skip_missing_targets" json:"skip_missing_targets" yaml:"skip_missing_targets"`
	SkipResolvedTargets *bool `bson:"skip_resolved_targets" json:"skip_resolved_targets" yaml:"skip_resolved_targets"`

	Runtime struct {
		hostname string
	} `bson:"-" json:"-" yaml:"-"`
}

func tryAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return abs
	}
	return path
}

func (disco *LinkDiscovery) Validate() (err error) {
	ec := &erc.Collector{}
	defer func() { err = ec.Resolve() }()
	defer ec.Recover()

	if len(disco.SearchPaths) == 0 {
		disco.SearchPaths = append(disco.SearchPaths, util.GetHomeDir())
	}

	for i := range disco.SearchPaths {
		disco.SearchPaths[i] = util.TryExpandHomeDir(disco.SearchPaths[i])
	}
	for i := range disco.IgnorePathPrefixes {
		disco.IgnorePathPrefixes[i] = util.TryExpandHomeDir(disco.IgnorePathPrefixes[i])
	}
	for i := range disco.IgnoreTargetPrefixes {
		disco.IgnoreTargetPrefixes[i] = util.TryExpandHomeDir(disco.IgnoreTargetPrefixes[i])
	}

	for idx := range disco.SearchPaths {
		path := disco.SearchPaths[idx]
		stat, err := os.Stat(disco.SearchPaths[idx])
		ec.Whenf(os.IsNotExist(err), "link search tree %q does not exist", path)
		ec.Whenf(!stat.IsDir() || stat.Mode().IsRegular(),
			"search tree must be either symlinks or directories, %s is %s", path, stat.Mode())
	}

	slices.Sort(disco.SearchPaths)
	disco.SearchPaths = slices.Compact(disco.SearchPaths)

	return err
}

func (disco *LinkDiscovery) ShouldSkipMissingTargets() bool {
	return stw.Deref(disco.SkipMissingTargets)
}

func (disco *LinkDiscovery) ShouldSkipResolvedTargets() bool {
	return stw.Deref(disco.SkipResolvedTargets)
}

func hasAnyPrefix(str string, prefixes []string) bool {
	for pf := range slices.Values(prefixes) {
		if strings.HasPrefix(str, pf) {
			return true
		}
	}
	return false
}

func (disco *LinkDiscovery) FindLinks() iter.Seq[LinkDefinition] {
	return func(yield func(LinkDefinition) bool) {
		for _, path := range disco.SearchPaths {
			for link := range libfun.FsWalkStream(libfun.FsWalkOptions{
				Path:                 path,
				SkipPermissionErrors: true,
				IgnoreMode:           stw.Ptr(fs.ModeDir),
			}, func(p string, dir fs.DirEntry) (*LinkDefinition, error) {
				if hasAnyPrefix(p, disco.IgnorePathPrefixes) {
					if dir.IsDir() {
						return nil, fs.SkipDir
					}
					return nil, nil
				}

				if dir.Type()&fs.ModeSymlink == 0 {
					return nil, nil
				}

				target, err := os.Readlink(p)
				if err != nil {
					if errors.Is(err, fs.ErrPermission) {
						return nil, nil
					}
					return nil, fmt.Errorf("error reading symbolic link %s: %w", path, err)
				}

				target = tryAbsPath(target)
				exists := util.FileExists(target)

				switch {
				case exists && disco.ShouldSkipResolvedTargets():
					return nil, nil
				case !exists && disco.ShouldSkipMissingTargets():
					return nil, nil
				case hasAnyPrefix(target, disco.IgnoreTargetPrefixes):
					return nil, nil
				}

				return &LinkDefinition{
					Name:         strings.TrimLeft(strings.ReplaceAll(target, string(filepath.Separator), "-"), "- _."),
					Target:       target,
					Path:         p,
					PathExists:   util.FileExists(p),
					TargetExists: exists,
				}, nil
			}) {
				if !yield(link) {
					return
				}
			}
		}
	}
}
