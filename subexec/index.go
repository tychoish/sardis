package subexec

import "github.com/tychoish/fun"

type Node struct {
	word     string
	comand   *Command
	children map[string]*Node
}

func makeTree(commands []Command) *Node {
	tree := &Node{children: make(map[string]*Node)}
	for idx := range commands {
		tree.Add(dotSpit(commands[idx].Name), 0, commands[idx])
	}
	return tree
}

func (n *Node) Add(path []string, depth int, cmd Command) {
	if len(path) == depth+1 {
		fun.Invariant.Ok(n.comand == nil, "command should not be defined")
		n.comand = &cmd
		return
	}

	ch := n.children[path[depth]]
	if ch == nil {
		n.children[path[depth]] = &Node{word: path[depth]}
	}

	ch.Add(path, depth+1, cmd)
}

func (n *Node) Find(id string) *Command { return n.Search(dotSpit(id), -1).comand }

func (n *Node) Search(path []string, depth int) *Node {
	if n == nil || depth+1 == len(path) {
		return n
	}

	return n.children[path[depth]].Search(path, depth+1)
}
