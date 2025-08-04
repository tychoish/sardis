package subexec

import (
	"encoding/json"
	"testing"

	"github.com/tychoish/fun/ft"
)

func TestTree(t *testing.T) {
	cmds := []Command{
		{Name: "category.group.name.subname", GroupName: "group"},
		{Name: "category.group.name.supername", GroupName: "group"},
		{Name: "category.group.nominal.subnominal", GroupName: "grope"},
		{Name: "category.group.nominal.internomincal", GroupName: "grope"},
	}

	node := NewCommandTree(cmds)
	t.Log(string(ft.Must(json.MarshalIndent(node, "", "     "))))

	notFound := node.FindCommand("category.group.name")
	if notFound != nil {
		t.Error("found cmd at wrong path")
	}

	found := node.FindCommand("category.group.name.subname")
	if found == nil {
		t.Error("shouldn't be found")
	}
}
