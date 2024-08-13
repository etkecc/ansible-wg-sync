package ansible

import (
	"sort"
	"strconv"
)

// Uniq removes duplicates from slice
func Uniq(slice []string) []string {
	uniq := map[string]struct{}{}
	for _, k := range slice {
		uniq[k] = struct{}{}
	}

	return MapKeys(uniq)
}

// MapKeys returns map keys only
func MapKeys[T string, V any](data map[string]V) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

// Unquote is wrapper around strconv.Unquote, but will return unmodified input string on error
func Unquote(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return unquoted
}
