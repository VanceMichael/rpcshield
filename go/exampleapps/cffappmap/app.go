//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

// Sink represents a downstream/outbound call site whose reachability we're testing.
func Sink(value int) error {
	return nil
}

// AFunctionWithUniqueName exercises cff.Map with a ctx-less, key/value-first task function
// signature, per cff.Map's documented API: "func(key someType, value someType) error { ... }"
// (no context.Context). cff.Map has no dedicated regex pattern in cffenricher.go at all (unlike
// cff.Slice), so it used to be silently dropped from the call graph unconditionally.
func AFunctionWithUniqueName(ctx context.Context, m map[string]int) error {
	return cff.Parallel(
		ctx,
		cff.Map(func(key string, value int) error {
			return Sink(value)
		}, m),
	)
}

func main() {
	ctx := context.Background()
	if err := AFunctionWithUniqueName(ctx, map[string]int{"a": 1}); err != nil {
		fmt.Println("Error:", err)
	}
}
