//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

// Sink represents a downstream/outbound call site whose reachability we're testing, reached only
// from the MapEnd closure (not the main Map task closure).
func Sink(n int) error {
	return nil
}

// AFunctionWithUniqueName exercises cff.MapEnd, analogous to cffappsliceend for cff.SliceEnd.
func AFunctionWithUniqueName(ctx context.Context, m map[string]int) error {
	return cff.Parallel(
		ctx,
		cff.Map(func(key string, value int) error {
			return nil
		}, m,
			cff.MapEnd(func(ctx context.Context) error {
				return Sink(len(m))
			}),
		),
	)
}

func main() {
	ctx := context.Background()
	if err := AFunctionWithUniqueName(ctx, map[string]int{"a": 1}); err != nil {
		fmt.Println("Error:", err)
	}
}
