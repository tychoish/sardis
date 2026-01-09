package execpath

import (
	"context"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/grip"
	"github.com/tychoish/libfun"
	"github.com/tychoish/sardis/util"
)

func FindAll(ctx context.Context) iter.Seq[string] {
	return irt.Chain(
		irt.Convert(
			irt.Unique(irt.Slice(filepath.SplitList(os.Getenv("PATH")))),
			func(path string) iter.Seq[string] {
				if !util.FileExists(path) {
					return nil
				}
				seq, closer := libfun.WalkDirIterator(path, func(p string, d fs.DirEntry) (*string, error) {
					if d.IsDir() || d.Type().Perm()&0o111 != 0 {
						return nil, nil
					}
					return stw.Ptr(strings.TrimSpace(p)), nil
				})
				defer func() { grip.Error(closer()) }()
				return irt.Keep(seq,
					func(in string) bool {
						if in == "" {
							return false
						}
						stat, err := os.Stat(in)
						if err != nil || os.IsNotExist(err) || stat.IsDir() {
							return false
						}
						return true
					},
				)
			},
		),
	)
}

type PacmanCommand string

const (
	PacmanCommandQuery PacmanCommand = "--query"
	PacmanCommandSync  PacmanCommand = "--sync"
)

type PacmanConf struct {
	Operation PacmanCommand
	Confirm   bool
	Query     []string
	Sync      []string
}

func (o *PacmanConf) withQuery(in []string) { o.Query = append(o.Query, in...) }
func (o *PacmanConf) withSync(in []string)  { o.Sync = append(o.Sync, in...) }

func (o *PacmanConf) validate() error {
	ec := &erc.Collector{}

	switch o.Operation {
	case PacmanCommandSync:
		ec.Whenf(len(o.Query) == 0, "cannot specify query options %v with sync command", o.Query)
	case PacmanCommandQuery:
		ec.Whenf(len(o.Sync) == 0, "cannot specify sync options %v with query command", o.Sync)
	}

	return ec.Resolve()
}

func (o *PacmanConf) apply(args ...PacmanArg) {
	for _, arg := range args {
		arg(o)
	}
}

type PacmanArg func(o *PacmanConf)

func WithOptions(in *PacmanConf) PacmanArg     { return func(o *PacmanConf) { *o = *in } }
func WithOperation(op PacmanCommand) PacmanArg { return func(o *PacmanConf) { o.Operation = op } }
func SetConfirm(state bool) PacmanArg          { return func(o *PacmanConf) { o.Confirm = state } }
func NoConfirm() PacmanArg                     { return SetConfirm(false) }
func WithConfirm() PacmanArg                   { return SetConfirm(true) }
func WithQueryArgs(a ...string) PacmanArg      { return func(o *PacmanConf) { o.withQuery(a) } }
func WithSyncArgs(a ...string) PacmanArg       { return func(o *PacmanConf) { o.withSync(a) } }

func Pacman(ctx context.Context, args ...PacmanArg) iter.Seq[string] { return nil }
