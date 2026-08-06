package xslice

// Fold applies over a slice by applying `op func(To, From) To` to the currently accumulated value and the current
// element of a slice returns an accumulated result of type TO.
//
// This function is nil safe.
//
// Example: sum a slice of ints
//
//	ints := []int{1, 2, 3}
//	sum := Fold(ints, 0, func(acc int, val int) int { return acc + val })
//
// Example: max of a slice of floats
//
//	floats := []float64{1.0, 2.0, 3.0}
//	max := Fold(ints, 0, max.Max)
func Fold[From any, To any](a []From, base To, op func(To, From) To) To {
	acc := base
	for _, e := range a {
		acc = op(acc, e)
	}
	return acc
}
