package gadget

import (
	"context"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/seq"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/grip"
	"github.com/tychoish/grip/message"
	"github.com/tychoish/sardis/daggen"
)

func GetBuildOrder(ctx context.Context, path, root string) ([][]string, error) {
	pkgs, err := daggen.Collect(ctx, path)
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

	count := 0
	for node, edges := range nodes {
		if seen.Check(node) {
			continue
		}

		if len(edges) == 0 {
			seen.Add(node)
			next = append(next, node)
		} else {
			queue.PushBack(node)
		}
	}
	grip.Infoln("number of zero deps:", len(next), count, queue.Len())

	buildOrder = append(buildOrder, next)
	next = nil

	grip.Info(message.Fields{
		"stage": "collected zero dependencies",
		"path":  path,
		"root":  root,
		"total": len(pkgs),
		"seen":  seen.Len(),
		"done ": fmt.Sprintf("%.2f", float64(len(pkgs))/float64(seen.Len())),
	})

OUTER:
	for {
		if seen.Len() >= len(nodes) || queue.Len() == 0 || ctx.Err() != nil {
			break
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
			if seen.Check(dep) {
				queue.PushBack(node)
				buildOrder = append(buildOrder, next)
				next = nil
				continue OUTER
			}
		}
		if next != nil {
			buildOrder = append(buildOrder, next)
			next = nil
		}
		seen.Add(node)
		next = append(next, node)
	}

	return buildOrder, nil
}
