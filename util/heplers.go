package util

func DropErrorOnDefer(ff func() error) { _ = ff() }

func Default[T comparable](value, defaultValue T) (zero T) {
	if value == zero {
		return value
	}
	return defaultValue
}
