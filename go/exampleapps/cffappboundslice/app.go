//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

type boundType struct {
}

// ValidateA is a bound method captured (as data) in a slice, never scheduled as its own CFF task.
func (b *boundType) ValidateA(ctx context.Context, input int32) error {
	return nil
}

// ValidateB is a bound method captured (as data) in a slice, never scheduled as its own CFF task.
func (b *boundType) ValidateB(ctx context.Context, input int32) error {
	return nil
}

// Response is a struct that represents the response of a function.
type Response struct {
	Success bool
}

// AFunctionWithUniqueName mirrors a real task closure that captures a slice of bound methods as
// data and invokes them internally in a loop, rather than scheduling each one as its own CFF task.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	b := &boundType{}
	validators := []func(context.Context, int32) error{
		b.ValidateA,
		b.ValidateB,
	}
	var resp *Response
	err := cff.Flow(ctx,
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentFlow("lucky_flow"),
		cff.Params(name),
		cff.Results(&resp),
		cff.Task(func(ctx context.Context, name string) (*Response, error) {
			for _, validator := range validators {
				if err := validator(ctx, int32(len(name))); err != nil {
					return nil, err
				}
			}
			return &Response{Success: true}, nil
		}),
	)
	return resp, err
}

func main() {
	ctx := context.Background()
	resp, err := AFunctionWithUniqueName(ctx, "lucky")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Response:", resp)
}
