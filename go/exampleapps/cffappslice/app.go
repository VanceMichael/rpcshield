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
func HasOrderedBefore(ctx context.Context, orderNum int) (bool, error) {
	if orderNum > 0 {
		return true, nil
	}
	return false, nil
}

// AFunctionWithUniqueName is a CffDemo.
func AFunctionWithUniqueName(ctx context.Context, name string) (*Response, error) {
	var resp *Response
	logger, _ := zap.NewProduction()

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
		cff.Slice(func(ctx context.Context, orderNume int) error {
			ojHasOrderedBefore, ojErr := HasOrderedBefore(ctx, orderNume)
			if ojHasOrderedBefore {
				return nil
			}
			return ojErr
		}, []int{0, 1, 2, 3},
		),
		cff.Task(func(ctx context.Context) error {
			resp = &Response{Success: false}
			return errors.New("error")
		}),
	)

	if err != nil {
		return nil, err
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
