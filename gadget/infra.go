package gadget

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
)

type IteratorWorker[T any] struct {
	fun.Iterator[T]
	adt.Atomic[fun.WorkerFunc]
}

func (iw IteratorWorker[T]) Close() error { return erc.Merge(iw.Iterator.Close(), iw.Get().Block()) }

func WithTiming(name string, op func()) {
	start := time.Now()
	defer func() {
		grip.Info(message.BuildPair().
			Pair("op", name).
			Pair("dur", time.Since(start)))
	}()

	op()
}

func prunePairs[K comparable, V any](iter fun.Iterator[fun.Pair[K, V]], limits ...K) fun.Iterator[fun.Pair[K, V]] {
	limit := set.NewUnordered[K]()
	fun.WorkerFunc(func(ctx context.Context) error {
		set.PopulateSet(ctx, limit, itertool.Slice(limits))
		return nil
	}).MustWait().Block()

	return fun.Filter(iter, func(pair fun.Pair[K, V]) bool { return limit.Check(pair.Key) })
}

type RipgrepArgs struct {
	Types         []string
	ExcludedTypes []string
	Regexp        string
	Path          string
	IgnoreFile    string
	Directories   bool
	Unique        bool
	Invert        bool
	Zip           bool
	WordRegexp    bool
}

func Ripgrep(ctx context.Context, jpm jasper.Manager, args RipgrepArgs) fun.Iterator[string] {
	args.Path = util.TryExpandHomedir(args.Path)
	sender := &bufsend{
		buffer: &bytes.Buffer{},
	}
	sender.SetPriority(level.Info)
	sender.SetName("ripgrep")
	sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

	cmd := []string{"rg",
		"--files-with-matches",
		"--line-buffered",
		"--color=never",
		"--trim",
	}
	if args.Invert {
		cmd = append(cmd, "--invert-match")
	}
	if args.IgnoreFile != "" {
		cmd = append(cmd, "--ignore-file", args.IgnoreFile)
	}
	if args.Zip {
		cmd = append(cmd, "--search-zip")
	}
	if args.WordRegexp {
		cmd = append(cmd, "--word-regexp")
	}
	for _, t := range args.Types {
		cmd = append(cmd, "--type", t)
	}
	for _, t := range args.ExcludedTypes {
		cmd = append(cmd, "--type-not", t)
	}
	cmd = append(cmd, "--regexp", args.Regexp)

	ec := &erc.Collector{}
	ec.Add(jpm.CreateCommand(ctx).
		Directory(args.Path).
		Add(cmd).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Error, grip.Sender()).
		Run(ctx))

	iter := LineIterator(strings.TrimSpace(sender.buffer.String()))

	if args.Directories {
		iter = fun.Transform(iter, func(in string) (string, error) { return filepath.Dir(in), nil })
	}

	if args.Unique {
		iter = itertool.Uniq(iter)
	}

	return iter
}

// WalkDirIterator provides an alternate fun.Iterator-based interface
// to filepath.WalkDir. The filepath.WalkDir runs in a go routnine,
// and calls a simpler walk function: where you can output an object,
// [in most cases a string of the path] but the function is generic.
//
// If the first value of the walk function is nil, then the item is
// skipped the walk will continue, otherwise--assuming that the error
// is non-nil, it is de-referenced and returned by the iterator.
func WalkDirIterator[T any](ctx context.Context, path string, fn func(p string, d fs.DirEntry) (*T, error)) fun.Iterator[T] {
	once := &sync.Once{}
	ec := &erc.Collector{}
	localCtx, cancel := context.WithCancel(ctx)
	var pipe chan T

	return fun.Generator(func(ctx context.Context) (T, error) {
		once.Do(func() {
			pipe = make(chan T)
			go func() {
				defer cancel()
				_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}

					out, err := fn(p, d)
					if err != nil {
						if !errors.Is(err, fs.SkipDir) && !errors.Is(err, fs.SkipAll) {
							ec.Add(err)
						}
						return err
					}
					if out == nil {
						return nil
					}

					select {
					case pipe <- *out:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})

			}()
		})
		if ec.HasErrors() {
			return fun.ZeroOf[T](), ec.Resolve()
		}
		select {
		case <-localCtx.Done():
			return fun.ZeroOf[T](), localCtx.Err()
		case out, ok := <-pipe:
			if !ok {
				return fun.ZeroOf[T](), io.EOF
			}
			return out, nil
		}
	})
}

func LineIterator(str string) fun.Iterator[string] {
	scanner := bufio.NewScanner(strings.NewReader(str))

	return fun.Generator(func(ctx context.Context) (string, error) {
		if !scanner.Scan() {
			return "", erc.Merge(io.EOF, scanner.Err())
		}
		return strings.TrimSpace(scanner.Text()), nil
	})
}

type bufsend struct {
	send.Base
	buffer *bytes.Buffer
}

func (b *bufsend) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		if _, err := b.buffer.Write([]byte(fmt.Sprintln(m.String()))); err != nil {
			b.ErrorHandler()(err, m)
		}

	}

}
