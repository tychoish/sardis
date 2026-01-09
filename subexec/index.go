package subexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"slices"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/stw"
	"github.com/tychoish/sardis/util"
)

type Node struct {
	word     string
	command  *Command
	children stw.Map[string, *Node]
}

func NewCommandTree(commands []Command) *Node {
	tree := NewNode()
	tree.add(slices.Values(commands))
	return tree
}

func NewTree(nodes []*Node) *Node {
	n := NewNode()
	for node := range slices.Values(nodes) {
		n.Push(node)
	}
	return n
}
func (conf *Configuration) Tree() *Node { return NewCommandTree(conf.ExportAllCommands()) }

func (n *Node) KeysAtLevel() []string {
	return util.SparseString(slices.Collect(maps.Keys(n.children)))
}

func (n *Node) Push(rhn *Node) bool {
	if rhn == nil {
		return false
	}

	n.children[rhn.word] = rhn
	return true
}

func (n *Node) Extend(ns iter.Seq[*Node]) *Node {
	for nn := range ns {
		n.Push(nn)
	}
	return n
}

func (n *Node) Len() int                  { return len(n.children) }
func (n *Node) Children() iter.Seq[*Node] { return maps.Values(n.children) }
func (n *Node) NarrowTo(key string) *Node { return n.children[key] }
func (n *Node) HasCommand() bool          { return n.command != nil }
func (n *Node) HasChidren() bool          { return n.children.Len() > 0 }
func (n *Node) Command() *Command         { return n.command }
func (n *Node) ID() string                { return n.word }

func (n *Node) MarshalJSON() ([]byte, error) {
	out := bytes.Buffer{}
	fmt.Fprintf(&out, `{"word":"%s","has_command":%t,"tree":%s}`, n.word, n.command != nil, string(erc.Must(json.Marshal(n.children))))
	return out.Bytes(), nil
}

func NewNode() *Node { return &Node{children: make(map[string]*Node)} }

func (n *Node) add(cmds iter.Seq[Command]) {
	var ok bool

	for cmd := range cmds {
		var prev *Node
		next := n

		parts := util.SparseString(util.DotSplit(cmd.NamePrime()))
		for idx, elem := range parts {
			prev = next

			if next, ok = prev.children[elem]; !ok {
				next = NewNode()
				next.word = elem
			}

			if idx+1 == len(parts) {
				cpcmd := cmd
				next.command = &cpcmd
			}

			prev.children[elem] = next
		}
	}
}

func (n *Node) Find(id string) *Node {
	return n.itersearch(
		util.SparseString(
			util.DotSplit(
				id,
			),
		),
	)
}

func (n *Node) FindCommand(id string) *Command {
	found := n.Find(id)
	if found == nil {
		return nil
	}
	return found.command
}

func (n *Node) itersearch(path []string) *Node {
	next := n

	for part := range slices.Values(path) {
		if _, ok := next.children[part]; ok {
			next = next.children[part]
			continue
		}
		return nil
	}

	return next
}

func (n *Node) Resolve() iter.Seq[Command] {
	queue := dt.List[*Node]{}
	queue.PushBack(n)

	return func(yield func(Command) bool) {
		for elem := queue.PopFront(); elem.Ok(); elem = queue.PopFront() {
			node := elem.Value()
			if node.command != nil && !yield(stw.Deref(node.command)) {
				return
			}
			for k := range node.children {
				queue.PushBack(node.children[k])
			}
		}
	}
}
