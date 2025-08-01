package util

import (
	"slices"
	"strings"

	"github.com/tychoish/fun/ft"
)

func DotJoin(elems ...string) string      { return DotJoinParts(elems) }
func DotJoinParts(elems []string) string  { return strings.Join(MakeSparse(elems), ".") }
func DotSplit(in string) []string         { return strings.Split(in, ".") }
func DotSplitN(in string, n int) []string { return strings.SplitN(in, ".", n) } // nolint:unused
func IsZero[T comparable](i T) bool       { var z T; return i == z }
func IsWhitespace(s string) bool          { return strings.TrimSpace(s) == "" }

func MakeSparse[T comparable](in []T) (out []T) { return NilWhenEmpty(slices.DeleteFunc(in, IsZero)) }
func NilWhenEmpty[T any](in []T) []T            { return ft.IfElse(len(in) > 0, in, nil) }
func SparseString(in []string) []string {
	return NilWhenEmpty(slices.DeleteFunc(in, JoinAnd(IsZero, IsWhitespace)))
}

func JoinAnd[T any](pfn ...func(T) bool) func(T) bool {
	return func(in T) bool {
		for fn := range slices.Values(pfn) {
			if !fn(in) {
				return false
			}
		}
		return true
	}
}

func JoinOr[T any](pfn ...func(T) bool) func(T) bool {
	return func(in T) bool {
		for fn := range slices.Values(pfn) {
			if fn(in) {
				return true
			}
		}
		return false
	}
}
