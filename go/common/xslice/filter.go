package xslice

// FilterInPlace will filter out any elements from "in" that do
// not match the provided "keep" predicate. The filtering is done in-place,
// i.e. no new memory is allocated. WARNING: The original contents
// of the input array are modified.
func FilterInPlace[T any](in []T, keep func(T) bool) []T {
	if len(in) == 0 {
		return in
	}
	// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}

	var zero T
	for i := len(out); i < len(in); i++ {
		in[i] = zero
	}

	return out
}

// Filter will filter out any elements from "in" that do
// not match the provided "keep" predicate. The filtering returns a new
// slice.
func Filter[T any](in []T, keep func(T) bool) []T {
	if len(in) == 0 {
		return nil
	}
	var (
		out = make([]T, len(in))
		i   int
	)
	for _, x := range in {
		if keep(x) {
			out[i] = x
			i++
		}
	}
	return out[:i:i]
}
