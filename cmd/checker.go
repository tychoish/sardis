package main

import (
	"fmt"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/sardis/util"
)

type ListNS[T any] struct{ dt.Slice[T] }

func NewList[T any](items ...T) *ListNS[T] {
	return &ListNS[T]{dt.NewSlice(items)}
}

func (ls ListNS[T]) Len() int { return ls.Slice.Len() }

func main() {
	fmt.Println(len(util.DotSplit("")))
	fmt.Println(len(util.DotSplit("..")))
	fmt.Println(len(util.DotSplit("  ")))
	fmt.Println((util.DotSplit("")))
	fmt.Println((util.DotSplit("..")))
	fmt.Println((util.DotSplit("  ")))
	fmt.Println(util.MakeSparse(util.DotSplit("")))
	fmt.Println(util.MakeSparse(util.DotSplit("..")))
	fmt.Println(util.MakeSparse(util.DotSplit("  ")))

	l := NewList("one", "two", "three")
	l.Push("four")
	fmt.Println(l, l.Len())
	alter(l)
	l.Push("seven")
	fmt.Println(l, l.Len())
}

func alter(ls *ListNS[string]) {
	fmt.Println("alter -->")
	ls.Push("seven")
	fmt.Println(ls, ls.Len())
	fmt.Println("<-- alter")
}
