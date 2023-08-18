package gadget

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
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
	sender := &bufsend{}
	sender.SetPriority(level.Info)
	sender.SetName("ripgrep")
	sender.SetErrorHandler(send.ErrorHandlerFromSender(grip.Sender()))

	cmd := dt.Slice[string]{
		"rg",
		"--files-with-matches",
		"--line-buffered",
		"--color=never",
		"--trim",
	}

	dt.Sliceify(args.Types).Observe(func(t string) { cmd.Append("--type", t) })
	dt.Sliceify(args.ExcludedTypes).Observe(func(t string) { cmd.Append("--type-not", t) })

	cmd.AppendWhen(args.Invert, "--invert-match")
	cmd.AppendWhen(args.IgnoreFile != "", "--ignore-file", args.IgnoreFile)
	cmd.AppendWhen(args.Zip, "--search-zip")
	cmd.AppendWhen(args.WordRegexp, "--word-regexp")

	cmd.Append("--regexp", args.Regexp)

	err := jpm.CreateCommand(ctx).
		Directory(args.Path).
		Add(cmd).
		SetOutputSender(level.Info, sender).
		SetErrorSender(level.Error, grip.Sender()).
		Run(ctx)

	iter := fun.Converter(func(in string) string {
		in = filepath.Join(args.Path, in)
		if args.Directories {
			return filepath.Dir(in)
		}
		return in
	}).Convert(fun.HF.Lines(&sender.buffer))
	iter.AddError(err)

	if args.Unique {
		return itertool.Uniq(iter)
	}

	return iter
}

// WalkDirIterator provides an alternate fun.Iterator-based interface
// to filepath.WalkDir. The filepath.WalkDir runs in a go routnine,
// and calls a simpler walk function: where you can output an object,
// [in most cases a string of the path] but the function is generic
// and you can convert or do other work as needed.
//
// If the first value of the walk function is nil, then the item is
// skipped the walk will continue, otherwise--assuming that the error
// is non-nil, it is de-referenced and returned by the iterator.
func WalkDirIterator[T any](ctx context.Context, path string, fn func(p string, d fs.DirEntry) (*T, error)) *fun.Iterator[T] {
	ec := &erc.Collector{}

	pipe := fun.Blocking(make(chan T))
	send := pipe.Processor()

	return pipe.Producer().PreHook(
		fun.Worker(
			func(ctx context.Context) error {
				return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
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

					return send(ctx, *out)
				})

			}).
			Operation(fun.HF.ErrorHandlerWithoutTerminating(ec.Add)).
			PostHook(pipe.Close).Once().Go(),
	).IteratorWithHook(erc.IteratorHook[T](ec))
}

type bufsend struct {
	send.Base
	buffer bytes.Buffer
}

func (b *bufsend) Send(m message.Composer) {
	if send.ShouldLog(b, m) {
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString(m.String())))
		fun.Invariant.Must(ft.IgnoreFirst(b.buffer.WriteString("\n")))
	}
}
