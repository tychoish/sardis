package execpath

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
)

func FindAll(ctx context.Context) *fun.Stream[string] {
	return fun.MergeStreams(
		fun.Convert(func(ctx context.Context, path string) (*fun.Stream[string], error) {
			if !util.FileExists(path) {
				return fun.VariadicStream(""), nil
			}

			return libfun.WalkDirIterator(path, func(p string, d fs.DirEntry) (*string, error) {
				if d.IsDir() || d.Type().Perm()&0o111 != 0 {
					return nil, nil
				}
				return ft.Ptr(strings.TrimSpace(p)), nil
			}), nil
		}).Parallel(
			itertool.Uniq(
				fun.SliceStream(
					filepath.SplitList(os.Getenv("PATH")),
				),
			),
		)).Filter(
		func(in string) bool {
			if in == "" {
				return false
			}
			stat, err := os.Stat(in)
			if err != nil || stat.IsDir() {
				return false
			}
			return true
		},
	).BufferParallel(runtime.NumCPU())
}
