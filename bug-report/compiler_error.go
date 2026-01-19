package fun

import "iter"

func ptr[T any](in T) *T   { return &in }
func deref[T any](in *T) T { return *in }

func repeatok[T any](limit int, op func() (T, bool)) func() (T, *bool) {
	return func() (zero T, _ *bool) {
		if limit > 0 {
			limit--
			v, ok := op()
			return v, ptr(ok)
		}
		return
	}
}

func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] {
	return func(yield func(iter.Seq[T]) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		for hasMore, gen := true, repeatok(num, next); num > 0 && hasMore && gen != nil && yield(
			func(inner func(T) bool) {
				for value, ok := gen(); ok != nil && hasMore = deref(ok); !hasMore || !inner(value); value, ok = gen() {
					continue
				}
			},
		); gen = repeatok(num, next) {
			continue
		}
	}
}
