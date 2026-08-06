//go:build cff

package main

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/cff"
	"go.uber.org/zap"
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

// HasOrderedBefore is a function that checks if the user has ordered before.
func HasOrderedBefore(ctx context.Context, orderNum int8) (bool, error) {
	if orderNum > 0 {
		return true, nil
	}
	return false, nil
}

// AFunctionWithUniqueName is a CffDemo.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	var resp *Response
	logger, _ := zap.NewProduction()
	int8Arg := int8(1)

	defer logger.Sync() // Flushes buffer, if any
	err := cff.Parallel(
		ctx,
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentParallel("lucky"),
		cff.ContinueOnError(false),
		cff.Concurrency(2),
		cff.Task(func(ctx context.Context) error {
			sw := stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
			fmt.Println(sw)
			return nil
		}),
		cff.Task(func(ctx context.Context) error {
			ojHasOrderedBefore, ojErr := HasOrderedBefore(ctx, int8Arg)
			if ojHasOrderedBefore {
				return nil
			}
			return ojErr
		}),
		cff.Task(func(ctx context.Context) error {
			resp = &Response{Success: false}
			return errors.New("error")
		}),
	)

	err1 := cff.Flow(ctx,
		cff.Params(int8Arg),
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentFlow("lucky_flow"),
		cff.Task(func(ctx context.Context, lucky int8) (*Response, error) {
			return &Response{Success: false}, nil
		}),
		cff.Task(func(ctx context.Context, _ *Response) error {
			return nil
		},
			cff.Invoke(true),
		),
	)

	if err != nil {
		return nil, err
	}

	if err1 != nil {
		return nil, err1

	}

	return resp, nil
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
