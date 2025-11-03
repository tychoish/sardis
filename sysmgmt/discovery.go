package sysmgmt

import (
	"errors"
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
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
)

type LinkDiscovery struct {
	// SearchPaths are a a list of paths/trees where the links contained within are (potentially?) managed.
	SearchPaths          dt.Slice[string] `bson:"search" json:"search" yaml:"search"`
	IgnorePathPrefixes   dt.Slice[string] `bson:"ignore_path_prefixes" json:"ignore_path_prefixes" yaml:"ignore_path_prefixes"`
	IgnoreTargetPrefixes dt.Slice[string] `bson:"ignore_target_prefixes" json:"ignore_target_prefixes" yaml:"ignore_target_prefixes"`

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

func (disco *LinkDiscovery) Validate() (err error) {
	ec := &erc.Collector{}
	defer func() { err = ec.Resolve() }()
	defer ec.Recover()

	if len(disco.SearchPaths) == 0 {
		disco.SearchPaths = append(disco.SearchPaths, util.GetHomeDir())
	}

	disco.SearchPaths = fnx.NewFuture(disco.SearchPaths.Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Resolve()
	disco.IgnorePathPrefixes = fnx.NewFuture(disco.IgnorePathPrefixes.Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Resolve()
	disco.IgnoreTargetPrefixes = fnx.NewFuture(disco.IgnoreTargetPrefixes.Stream().Transform(fnx.MakeConverter(util.TryExpandHomeDir)).Slice).Resolve()

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
func (disco *LinkDiscovery) ShouldSkipMissingTargets() bool { return ft.Ref(disco.SkipMissingTargets) }

func (disco *LinkDiscovery) ShouldSkipResolvedTargets() bool {
	return ft.Ref(disco.SkipResolvedTargets)
}

func hasAnyPrefix(str string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return false
	}

	for pf := range slices.Values(prefixes) {
		if strings.HasPrefix(str, pf) {
			return true
		}
	}
	return false
}

func (disco *LinkDiscovery) FindLinks() *fun.Stream[*LinkDefinition] {
	return fun.Convert(fnx.MakeConverter(func(in dt.Tuple[string, string]) *LinkDefinition {
		return &LinkDefinition{
			Name:   strings.TrimLeft(strings.ReplaceAll(in.One, string(filepath.Separator), "-"), "- _."),
			Target: in.One,
			Path:   in.Two,
			// RequireSudo: strings.HasPrefix(in.Two, disco.Runtime.hostname),
		}
	})).Stream(fun.MergeStreams(fun.Convert(fnx.MakeConverter(func(path string) *fun.Stream[dt.Tuple[string, string]] {
		path = tryAbsPath(path)
		return libfun.FsWalkStream(libfun.FsWalkOptions{
			Path:                 path,
			SkipPermissionErrors: true,
			IgnoreMode:           ft.Ptr(fs.ModeDir),
		}, func(p string, dir fs.DirEntry) (*dt.Tuple[string, string], error) {
			p = tryAbsPath(filepath.Join(path, p))

			if dir.Type()&fs.ModeSymlink == 0 {
				return nil, nil
			}

			if hasAnyPrefix(p, disco.IgnorePathPrefixes) {
				if dir.Type()&fs.ModeDir != 0 {
					return nil, fs.SkipDir
				}
				return nil, nil
			}

			target, err := os.Readlink(p)
			switch {
			case err == nil:
				target = tryAbsPath(target)
				if hasAnyPrefix(target, disco.IgnoreTargetPrefixes) {
					return nil, nil
				}
			case errors.Is(err, fs.ErrPermission):
				return nil, nil
			case err != nil:
				grip.Debug(message.Fields{
					"link":   p,
					"target": target,
					"err":    err,
				})
				return nil, nil
			}
			grip.Info(message.Fields{"link": p, "target": target})
			return ft.Ptr(dt.MakeTuple(target, p)), nil
		})
	})).Stream(fun.SliceStream(disco.SearchPaths))))
}
