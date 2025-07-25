package subexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)


type Node struct {
	word     string
	command  *Command
	children dt.Map[string, *Node]
}

func NewTree(commands []Command) *Node {
	tree := makeNode()
	tree.add(commands)
	return tree
}


func (n *Node) MarshalJSON() ([]byte, error) {
	out := bytes.Buffer{}
	fmt.Fprintf(&out, `{"word":"%s","has_command":%t,"tree":%s}`, n.word, n.command != nil, string(ft.Must(json.Marshal(n.children))))
	return out.Bytes(), nil
}

func makeNode() *Node { return &Node{children: make(map[string]*Node)} }

func (n *Node) add(cmds []Command) {
	var ok bool

	for cmd := range slices.Values(cmds) {
		parts := dotSplit(cmd.NamePrime())

		var prev *Node
		next := n

		for idx, elem := range parts {
			prev = next

			if next, ok = prev.children[elem]; !ok {
				next = makeNode()
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

func (n *Node) Find(id string) *Node { return n.itersearch(dotSplit(id)) }

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

func (n *Node) Resolve() []Command {
	out := []Command{}

	queue := dt.List[*Node]{}
	queue.PushBack(n)

	for elem := queue.PopFront(); elem.Ok(); queue = queue.PopFront() {
		node := elem.Value()
		if node.command != nil {
			out = append(out, ft.Ref(node.command))
		}
		for k := range node.children {
			queue.PushBack(node.children[k])
		}
	}

	return out
}
