//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

// Sink represents a downstream/outbound call site whose reachability we're testing. It must NOT
// be reachable in the analyzed graph if the cffContextCallingFilter-drop hypothesis is correct.
func Sink(item int) error {
	return nil
}

// AFunctionWithUniqueName exercises cff.Slice with a ctx-less, index-first task function
// signature -- a shape explicitly documented as valid by cff.Slice's doc comment ("Optionally, a
// slice index of type int can be provided as a first ... parameter: func(idx int, item SomeType)
// { ... }"), and the exact shape used by ads-campaign-manager's real ValidateAdGroups.
func AFunctionWithUniqueName(ctx context.Context, items []int) error {
	return cff.Parallel(
		ctx,
		cff.Slice(func(idx int, item int) error {
			return Sink(item)
		}, items),
	)
}

func main() {
	ctx := context.Background()
	if err := AFunctionWithUniqueName(ctx, []int{0, 1, 2, 3}); err != nil {
		fmt.Println("Error:", err)
	}
}
