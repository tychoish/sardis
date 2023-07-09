package gadget

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/level"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/grip/send"
	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/util"
)

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

func Ripgrep(ctx context.Context, jpm jasper.Manager, args RipgrepArgs) *fun.Iterator[string] {
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

	iter := itertool.Lines(sender.buffer)

	iter = fun.ConvertIterator(iter, func(_ context.Context, in string) (string, error) { return filepath.Join(args.Path, in), nil })

	if args.Directories {
		iter = fun.ConvertIterator(iter, func(_ context.Context, in string) (string, error) { return filepath.Dir(in), nil })
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
func WalkDirIterator[T any](ctx context.Context, path string, fn func(p string, d fs.DirEntry) (*T, error)) *fun.Iterator[T] {
	once := &sync.Once{}
	ec := &erc.Collector{}
	var pipe chan T

	return fun.Generator(func(ctx context.Context) (T, error) {
		once.Do(func() {
			pipe = make(chan T)
			go func() {
				defer close(pipe)
				_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
					if err != nil {
						return fs.SkipAll
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

					return fun.Blocking(pipe).Send().Write(ctx, *out)
				})
			}()
		})
		if ec.HasErrors() {
			var zero T
			return zero, ec.Resolve()
		}

		return fun.Blocking(pipe).Receive().Read(ctx)
	})
}

// TODO: move to grip when we have a risky.Ignore
type bufsend struct {
	send.Base
	buffer *bytes.Buffer
}

func (b *bufsend) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString(m.String())))
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString("\n")))
	}
}
