package util

import (
	"iter"
	"slices"
	"strings"
)

func DotJoin(elems ...string) string            { return DotJoinParts(elems) }
func DotJoinParts(elems []string) string        { return strings.Join(MakeSparse(elems), ".") }
func DotSplit(in string) []string               { return strings.Split(in, ".") }
func DotSplitN(in string, n int) []string       { return strings.SplitN(in, ".", n) } // nolint:unused
func IsZero[T comparable](i T) bool             { var z T; return i == z }
func IsWhitespace(s string) bool                { return strings.TrimSpace(s) == "" }
func MakeSparse[T comparable](in []T) (out []T) { return NilWhenEmpty(slices.DeleteFunc(in, IsZero)) }
func NilWhenEmpty[T any](in []T) []T {
	if len(in) > 0 {
		return in
	}
	return nil
}

func Narrow[T any](indexes []int, source []T) []T {
	out := make([]T, 0, len(indexes))
	for idx := range slices.Values(indexes) {
		out = append(out, source[idx])
	}
	return out
}

func MakeSparseRefs[T any](in iter.Seq[*T]) iter.Seq[T] {
	return Convert(
		func(v *T) T {
			return *v
		},
		Filter(
			in,
			func(i *T) bool {
				return i != nil
			}),
	)
}

func SparseString(in []string) []string {
	return NilWhenEmpty(slices.DeleteFunc(in, JoinAnd(IsZero, IsWhitespace)))
}

func Filter[T any](it iter.Seq[T], pfn func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range it {
			if pfn(value) && !yield(value) {
				return
			}
		}
	}
}

func Convert[IN any, OUT any](mpf func(IN) OUT, input iter.Seq[IN]) iter.Seq[OUT] {
	return func(yield func(val OUT) bool) {
		for val := range input {
			if !yield(mpf(val)) {
				return
			}
		}
	}
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
