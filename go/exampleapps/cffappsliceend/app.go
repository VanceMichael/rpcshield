//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

// Sink represents a downstream/outbound call site whose reachability we're testing, reached only
// from the SliceEnd closure (not the main Slice task closure).
func Sink(n int) error {
	return nil
}

// AFunctionWithUniqueName exercises cff.SliceEnd. Under the current internal codegen's hybrid
// shape, cff.SliceEnd's closure is wrapped in a synthesized _cffSliceEnd..._<line>_<col> call
// nested as a trailing argument to the _cffSlice..._<line>_<col> wrapper -- processCFFTaskCall
// used to only inspect the wrapper's first argument (the main per-item closure), so the SliceEnd
// closure (and Sink through it) was never found.
func AFunctionWithUniqueName(ctx context.Context, items []int) error {
	return cff.Parallel(
		ctx,
		cff.Slice(func(item int) error {
			return nil
		}, items,
			cff.SliceEnd(func(ctx context.Context) error {
				return Sink(len(items))
			}),
		),
	)
}

func main() {
	ctx := context.Background()
	if err := AFunctionWithUniqueName(ctx, []int{0, 1, 2, 3}); err != nil {
		fmt.Println("Error:", err)
	}
}
