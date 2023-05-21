package gadget

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
)

type BuildOrder struct {
	Order    [][]string
	Packages Packages
	Path     string
}

func (bo *BuildOrder) Narrow(limits set.Set[string]) *BuildOrder {
	out := &BuildOrder{Packages: bo.Packages, Path: bo.Path}

	for _, group := range bo.Order {
		ng := []string{}
		for idx := range group {
			if limits.Check(group[idx]) {
				ng = append(ng, group[idx])
			}
		}
		if len(ng) > 0 {
			out.Order = append(out.Order, ng)
		}
	}

	return out
}

func GetBuildOrder(ctx context.Context, path string) (*BuildOrder, error) {
	pkgs, err := Collect(ctx, path)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no modules found in %q", path)
	}

	index := pkgs.IndexByPackageName()
	nodes := pkgs.Graph()

	seen := set.NewUnordered[string]()
	buildOrder := [][]string{}

	next := []string{}
	queue := &seq.List[string]{}

	iter := nodes.Iterator()
	for iter.Next(ctx) {
		item := iter.Value()
		node := item.Key
		edges := item.Value
		info, ok := index[node]
		fun.Invariant(ok, "bad index")

		if len(edges) == 0 && len(info.Dependencies) == 0 {
			next = append(next, node)
		} else {
			queue.PushBack(node)
		}
	}
	sort.Strings(next)
	set.PopulateSet(ctx, seen, itertool.Slice(next))
	buildOrder = append(buildOrder, next)
	next = nil
	grip.Debug(message.Fields{
		"stage": "collected zero dependencies",
		"path":  path,
		"total": len(pkgs),
		"seen":  seen.Len(),
		"done":  fmt.Sprintf("%.2f%%", float64(seen.Len())/float64(len(pkgs))*100),
	})

	var runsSinceProgress int
	iters := 0
OUTER:
	for {
		iters++
		if len(pkgs) == 0 || seen.Len() >= len(nodes) || queue.Len() == 0 || ctx.Err() != nil {
			break
		}
		if runsSinceProgress == queue.Len() && len(next) > 0 {
			sort.Strings(next)
			set.PopulateSet(ctx, seen, itertool.Slice(next))
			buildOrder = append(buildOrder, next)
			next = nil
			runsSinceProgress = 0
		}

		if runsSinceProgress >= queue.Len()*10 {
			err := errors.New("irresolveable dependency")
			grip.Warning(message.Fields{
				"err":    err,
				"pkgs":   len(pkgs),
				"queue":  queue.Len(),
				"groups": len(buildOrder),
				"seen":   seen.Len(),
				"next":   len(next),
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

		for _, dep := range info.Dependencies {
			if !seen.Check(dep) {
				queue.PushBack(node)
				runsSinceProgress++
				continue OUTER
			}
		}

		next = append(next, node)
		runsSinceProgress = 0
	}

	if len(next) > 0 {
		buildOrder = append(buildOrder, next)
	}
	return &BuildOrder{
		Order:    buildOrder,
		Packages: pkgs,
		Path:     path,
	}, nil
}
