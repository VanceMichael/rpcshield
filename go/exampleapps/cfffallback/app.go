//go:build cff

package main

import (
	"context"
	"fmt"

	"go.uber.org/cff"
)

func generalCall(ctx context.Context, input float64) (int32, error) {
	return int32(10), nil
}

// Response is a struct that represents the response of a function.
type Response struct {
	Success bool
}

// AFunctionWithUniqueName synchronously creates and saves a new CffDemo.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	var resp *Response
	err := cff.Flow(ctx,
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentFlow("lucky_flow"),
		cff.Params(name, genCallInput),
		cff.Results(&resp),
		cff.Task(func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}),
		cff.Task(func(ctx context.Context, lucky int32) (bool, error) {
			return false, nil
		}, cff.FallbackWith(true)),
		cff.Task(func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Response, error) {
			return &Response{Success: won}, nil
		}),
		cff.Task(generalCall, cff.FallbackWith(int32(42))),
		cff.Task(func(ctx context.Context, _ *Response) error {
			return nil
		},
			cff.Invoke(true),
		),
	)
	return resp, err
}

type stringWrapper struct {
	name string
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
