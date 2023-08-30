package gadget

import (
	"context"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

var fileCache = &adt.Map[string, []byte]{}

func PopulateCache(ctx context.Context, root string) error {
	start := time.Now()
	initSize := fileCache.Len()
	var (
		countFilesConsidered = &atomic.Int64{}
		countFilesReturned   = &atomic.Int64{}
		countFilesChecked    = &atomic.Int64{}
		countFilesRead       = &atomic.Int64{}
	)

	goFilesIter := WalkDirIterator(ctx, root, func(path string, d fs.DirEntry) (*string, error) {
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
	return itertool.ParallelForEach(ctx, goFilesIter, func(ctx context.Context, path string) error {
		countFilesChecked.Add(1)
		if fileCache.Check(path) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		countFilesRead.Add(1)
		fileCache.Store(path, data)
		return nil
	}, fun.WorkerGroupConfSet(&fun.WorkerGroupConf{NumWorkers: runtime.NumCPU(), ContinueOnError: true}))

}
