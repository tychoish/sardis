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
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
)

func WithTiming(name string, op func()) {
	start := time.Now()
	defer func() {
		grip.Info(message.BuildPair().
			Pair("op", name).
			Pair("dur", time.Since(start)))
	}()

	op()
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
