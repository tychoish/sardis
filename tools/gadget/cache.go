package gadget

import (
	"context"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
)

var fileCache = &stw.Map[string, []byte]{}

func PopulateCache(ctx context.Context, root string) (err error) {
	start := time.Now()
	initSize := fileCache.Len()
	var (
		countFilesConsidered = &atomic.Int64{}
		countFilesReturned   = &atomic.Int64{}
		countFilesChecked    = &atomic.Int64{}
		countFilesRead       = &atomic.Int64{}
	)
	ec := &erc.Collector{}
	defer func() { err = ec.Resolve() }()
	goFilesIter, closer := libfun.WalkDirIterator(root, func(path string, d fs.DirEntry) (*string, error) {
		countFilesConsidered.Add(1)
		if d.IsDir() && d.Name() == ".git" {
			return nil, fs.SkipDir
		}

		if fileCache.Check(path) || !strings.HasSuffix(path, ".go") {
			return nil, nil
		}
		countFilesReturned.Add(1)
		return &path, nil
	})
	defer func() {
		ec.Push(closer())
		nowSize := fileCache.Len()
		grip.Info(message.Fields{
			"path":             root,
			"cached":           nowSize,
			"added":            nowSize - initSize,
			"dur":              time.Since(start),
			"files_considered": countFilesConsidered.Load(),
			"files_returned":   countFilesRead.Load(),
			"files_checked":    countFilesChecked.Load(),
			"files_read":       countFilesRead.Load(),
		})
	}()
	for path := range goFilesIter {
		countFilesChecked.Add(1)
		if fileCache.Check(path) {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer util.DropErrorOnDefer(f.Close)

		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		countFilesRead.Add(1)
		fileCache.Store(path, data)
	}
	return nil
}
