package ansible

import (
	"errors"
	"os"
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

// FileExists checks if file exists
func FileExists(f string) bool {
	_, err := os.Stat(f)
	if err == nil {
		return true
	}

	return !errors.Is(err, os.ErrNotExist)
}

// Unquote is wrapper around strconv.Unquote, but will return unmodified input string on error
func Unquote(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return unquoted
}
