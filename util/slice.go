package util

func SliceContains[T comparable](slice []T, item T) bool {
	for idx := range slice {
		if slice[idx] == item {
			return true
		}
	}

	return false
}
