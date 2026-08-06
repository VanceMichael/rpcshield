//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

// SinkOne, SinkTwo, SinkThree represent distinct downstream/outbound call sites whose
// reachability we're testing -- one per task in the mixed cff.Parallel below, so we can tell
// whether findCFFTaskFuncs correctly recovers ALL of them without cross-contamination.
func SinkOne() error {
	return nil
}

func SinkTwo(item int) error {
	return nil
}

func SinkThree() error {
	return nil
}

type receiver struct{}

// BoundMethodTask is used as a bound-method value (receiver.BoundMethodTask) passed directly as a
// cff.Task argument, rather than a literal func literal -- exercising the *ssa.MakeClosure "bound
// method" wrapper shape under the hybrid codegen's generic-wrapping pattern.
func (receiver) BoundMethodTask(ctx context.Context) error {
	return SinkThree()
}

// AFunctionWithUniqueName exercises a single cff.Parallel with THREE mixed task/slice arguments:
// a plain cff.Task, a cff.Slice, and a cff.Task passed as a bound method value (not a literal
// closure).
func AFunctionWithUniqueName(ctx context.Context, items []int) error {
	r := receiver{}
	return cff.Parallel(
		ctx,
		cff.Task(func(ctx context.Context) error {
			return SinkOne()
		}),
		cff.Slice(func(i int, item int) error {
			return SinkTwo(item)
		}, items),
		cff.Task(r.BoundMethodTask),
	)
}

// AFunctionWithUniqueNameTwoSlices exercises a single cff.Parallel with TWO cff.Slice arguments
// over slices of the SAME element type ([]int), to check whether the two synthesized
// _cffSliceXXX generic instantiations (structurally identical modulo the closure/items they close
// over) get correctly kept separate by findCFFTaskFuncs, or whether closure recovery
// cross-contaminates.
func AFunctionWithUniqueNameTwoSlices(ctx context.Context, itemsA []int, itemsB []int) error {
	return cff.Parallel(
		ctx,
		cff.Slice(func(i int, item int) error {
			return SinkOne()
		}, itemsA),
		cff.Slice(func(i int, item int) error {
			return SinkTwo(item)
		}, itemsB),
	)
}

func main() {
	ctx := context.Background()
	if err := AFunctionWithUniqueName(ctx, []int{0, 1, 2, 3}); err != nil {
		fmt.Println("Error:", err)
	}
	if err := AFunctionWithUniqueNameTwoSlices(ctx, []int{0, 1}, []int{2, 3}); err != nil {
		fmt.Println("Error:", err)
	}
}
