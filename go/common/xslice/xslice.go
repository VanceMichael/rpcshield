package xslice

import (
	"slices"
)

// Unique filters out any duplicate elements of input in order and returns the result.
// If no duplicates were filtered, input is returned verbatim.
// Otherwise, Unique returns a copy of input with duplicate elements removed.
// Elements are always returned in the same order as input.
func Unique[T comparable](input []T) []T {
	var (
		seen   = make(map[T]struct{}, len(input))
		result []T
		last   int
	)

	for i, val := range input {
		if _, ok := seen[val]; ok {
			if i > last {
				result = append(result, input[last:i]...)
			}
			last = i + 1 // move the window forward, skipping this element
		} else {
			seen[val] = struct{}{}
		}
	}

	if last == 0 {
		return input
	}

	return append(result, input[last:]...)
}

// AllMatch returns true if all of the elements satisfy the predicate function or if the slice is empty, otherwise false.
func AllMatch[T any](values []T, predicate func(T) bool) bool {
	return !slices.ContainsFunc(values, func(t T) bool {
		return !predicate(t)
	})
}
