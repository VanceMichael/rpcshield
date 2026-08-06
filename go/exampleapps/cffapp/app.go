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

type boundType struct {
}

func (b *boundType) boundMethod(ctx context.Context, input int32) (int16, error) {
	return int16(0), nil
}

func (b *boundType) DoSomething(ctx context.Context, input int16) (int8, error) {
	return int8(5), nil
}

// Response is a struct that represents the response of a function.
type Response struct {
	Success bool
}

type someIface interface {
	DoSomething(ctx context.Context, input int16) (int8, error)
}

// AFunctionWithUniqueName synchronously creates and saves a new CffDemo.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	genCallInput := 21.3
	bm := &boundType{}
	var resp *Response
	iface := &boundType{}
	var int8Result int8
	err := cff.Flow(ctx,
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentFlow("lucky_flow"),
		cff.Params(name, genCallInput),
		cff.Results(&resp, &int8Result),
		cff.Task(func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}),
		cff.Task(func(ctx context.Context, lucky int32) (bool, error) {
			return false, nil
		}),
		cff.Task(func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Response, error) {
			return &Response{Success: won}, nil
		}),
		cff.Task(generalCall),
		cff.Task(bm.boundMethod),
		cff.Task(iface.DoSomething),
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
