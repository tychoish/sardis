package gadget

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis/daggen"
)

type BuildOrder struct {
	Order    [][]string
	Packages daggen.Packages
}

func GetBuildOrder(ctx context.Context, path string) (*BuildOrder, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pkgs, err := daggen.Collect(ctx, path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no modules found in %q", path)
	}

	grip.Info("number of packages")
	index := pkgs.IndexByPackageName()
	nodes := pkgs.Graph()

	seen := set.NewUnordered[string]()
	buildOrder := [][]string{}

	next := []string{}
	queue := &seq.List[string]{}

	count := 0
	for node, edges := range nodes {
		if len(edges) == 0 {
			next = append(next, node)
		} else {
			queue.PushBack(node)
		}
	}
	grip.Infoln("number of zero deps:", len(next), count, queue.Len(), seen.Len())

	set.PopulateSet(ctx, seen, itertool.Slice(next))
	buildOrder = append(buildOrder, next)
	next = nil

	grip.Info(message.Fields{
		"stage": "collected zero dependencies",
		"path":  path,
		"total": len(pkgs),
		"seen":  seen.Len(),
		"done ": fmt.Sprintf("%.2f", float64(len(pkgs))/float64(seen.Len())),
	})

	var runsSinceProgress int
	for {
		if len(pkgs) == 0 || seen.Len() >= len(nodes) || queue.Len() == 0 || ctx.Err() != nil {
			break
		}
		if runsSinceProgress >= queue.Len()*10 {
			err := errors.New("irresolveable dependency")
			grip.Warning(message.Fields{
				"err":    err,
				"pkgs":   len(pkgs),
				"queue":  queue.Len(),
				"groups": len(buildOrder),
				"seen":   seen.Len(),
			})
			return nil, errors.New("irresolveable dependency")
		}

		elem := queue.PopFront()
		if elem == nil {
			break
		}
		node := elem.Value()
		if seen.Check(node) {
			continue
		}

		info, ok := index[node]
		fun.Invariant(ok)

		depsMet := 0
		for _, dep := range info.Dependencies {
			if seen.Check(dep) {
				depsMet++
			}
		}
		if len(info.Dependencies) == depsMet {
			next = append(next, node)
			runsSinceProgress = 0
			continue
		} else {
			queue.PushBack(node)
			runsSinceProgress++
		}

		if len(next) > 0 {
			set.PopulateSet(ctx, seen, itertool.Slice(next))
			buildOrder = append(buildOrder, next)
			next = nil
		}
	}
	if len(next) > 0 {
		buildOrder = append(buildOrder, next)
	}

	return &BuildOrder{
		Order:    buildOrder,
		Packages: pkgs,
	}, nil

}
