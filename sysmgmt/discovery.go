package sysmgmt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
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
	return ft.IfElse(ers.IsOk(err), abs, path)
}

func (disco *LinkDiscovery) Validate() error {
	ec := &erc.Collector{}

	if len(disco.SearchPaths) == 0 {
		disco.SearchPaths = append(disco.SearchPaths, util.GetHomeDir())
	}

	disco.SearchPaths = fnx.NewFuture(dt.NewSlice(disco.SearchPaths).Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Force().Resolve()
	disco.IgnorePathPrefixes = fnx.NewFuture(dt.NewSlice(disco.IgnorePathPrefixes).Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Force().Resolve()
	disco.IgnoreTargetPrefixes = fnx.NewFuture(dt.NewSlice(disco.IgnoreTargetPrefixes).Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Force().Resolve()

	for idx := range disco.SearchPaths {
		path := disco.SearchPaths[idx]
		stat, err := os.Stat(disco.SearchPaths[idx])
		ec.Whenf(os.IsNotExist(err), "link search tree %q does not exist", path)
		ec.Whenf(!stat.IsDir() || stat.Mode().IsRegular(),
			"search tree must be either symlinks or directories, %s is %s", path, stat.Mode())
	}
	return ec.Resolve()
}
func (disco *LinkDiscovery) ShouldSkipMissingTargets() bool  { return ft.Ref(disco.SkipMissingTargets) }
func (disco *LinkDiscovery) ShouldSkipResolvedTargets() bool { return ft.Ref(disco.SkipMissingTargets) }

func (disco *LinkDiscovery) FindLinks() *fun.Stream[*LinkDefinition] {
	return fun.Convert(fnx.MakeConverter(func(in dt.Tuple[string, string]) *LinkDefinition {
		return &LinkDefinition{
			Name:        strings.TrimLeft(strings.ReplaceAll(in.One, string(filepath.Separator), "-"), "- _."),
			Target:      in.One,
			Path:        in.Two,
			RequireSudo: strings.HasPrefix(in.Two, disco.Runtime.hostname),
		}
	})).Stream(fun.MergeStreams(fun.Convert(fnx.MakeConverter(func(path string) *fun.Stream[dt.Tuple[string, string]] {
		return libfun.WalkDirIterator(path, func(p string, dir fs.DirEntry) (*dt.Tuple[string, string], error) {
			// Check if the file is a symbolic link
			if dir.Type()&fs.ModeSymlink != 0 {
				target, err := os.Readlink(p)
				if err != nil {
					if errors.Is(err, fs.ErrPermission) {
						return nil, nil
					}
					return nil, fmt.Errorf("error reading symbolic link %s: %w", path, err)
				}
				target = tryAbsPath(target)

				if util.FileExists(target) {
					if disco.ShouldSkipResolvedTargets() {
						return nil, nil
					}
				} else if disco.ShouldSkipMissingTargets() {
					return nil, nil
				}

				return ft.Ptr(dt.MakeTuple(target, p)), nil
			}

			return nil, nil
		})
	})).Stream(fun.SliceStream(disco.SearchPaths)))).Filter(func(link *LinkDefinition) bool {
		for ignore := range slices.Values(disco.IgnorePathPrefixes) {
			if strings.HasPrefix(link.Path, ignore) {
				return false
			}
		}

		for ignore := range slices.Values(disco.IgnoreTargetPrefixes) {
			if strings.HasPrefix(link.Target, ignore) {
				return false
			}
		}

		return true
	})
}
