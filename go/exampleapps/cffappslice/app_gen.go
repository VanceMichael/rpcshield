//go:build !cff

package main

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

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
	err := func() (err error) {

		_53_3 := ctx

		_54_19 := cff.NopEmitter()

		_55_26 := "lucky"

		_56_23 := false

		_57_19 := 2

		_58_12 := func(ctx context.Context) error {
			sw := stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
			fmt.Println(sw)
			return nil
		}

		_63_13 := func(ctx context.Context, orderNume int) error {
			ojHasOrderedBefore, ojErr := HasOrderedBefore(ctx, orderNume)
			if ojHasOrderedBefore {
				return nil
			}
			return ojErr
		}

		_69_6 := []int{0, 1, 2, 3}

		_71_12 := func(ctx context.Context) error {
			resp = &Response{Success: false}
			return errors.New("error")
		}
		ctx := _53_3
		emitter := cff.EmitterStack(_54_19)

		var (
			parallelInfo = &cff.ParallelInfo{
				Name:   _55_26,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cffappslice/app.go",
				Line:   52,
				Column: 9,
			}
			directiveInfo = &cff.DirectiveInfo{
				Name:      parallelInfo.Name,
				Directive: cff.ParallelDirective,
				File:      parallelInfo.File,
				Line:      parallelInfo.Line,
				Column:    parallelInfo.Column,
			}
			parallelEmitter = emitter.ParallelInit(parallelInfo)

			schedInfo = &cff.SchedulerInfo{
				Name:      parallelInfo.Name,
				Directive: cff.ParallelDirective,
				File:      parallelInfo.File,
				Line:      parallelInfo.Line,
				Column:    parallelInfo.Column,
			}

			// possibly unused
			_ = parallelInfo
			_ = directiveInfo
		)

		startTime := time.Now()
		defer func() { parallelEmitter.ParallelDone(ctx, time.Since(startTime)) }()

		schedEmitter := emitter.SchedulerInit(schedInfo)

		sched := cff.NewScheduler(
			cff.SchedulerParams{
				Concurrency: _57_19, Emitter: schedEmitter,
				ContinueOnError: _56_23,
			},
		)

		var tasks []*struct {
			emitter cff.TaskEmitter
			fn      func(context.Context) error
			ran     cff.AtomicBool
		}
		defer func() {
			for _, t := range tasks {
				if !t.ran.Load() {
					t.emitter.TaskSkipped(ctx, err)
				}
			}
		}()

		// github.com/uber-research/rpcshield/go/exampleapps/cffappslice/app.go:58:12
		task0 := new(struct {
			emitter cff.TaskEmitter
			fn      func(context.Context) error
			ran     cff.AtomicBool
		})
		task0.emitter = cff.NopTaskEmitter()
		task0.fn = func(ctx context.Context) (err error) {
			taskEmitter := task0.emitter
			startTime := time.Now()
			defer func() {
				if task0.ran.Load() {
					taskEmitter.TaskDone(ctx, time.Since(startTime))
				}
			}()

			defer func() {
				recovered := recover()
				if recovered != nil {
					taskEmitter.TaskPanic(ctx, recovered)
					err = &cff.PanicError{
						Value:      recovered,
						Stacktrace: debug.Stack(),
					}
				}
			}()

			defer task0.ran.Store(true)

			err = _58_12(ctx)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return
			}
			taskEmitter.TaskSuccess(ctx)
			return
		}

		sched.Enqueue(ctx, cff.Job{
			Run: task0.fn,
		})
		tasks = append(tasks, task0)

		// github.com/uber-research/rpcshield/go/exampleapps/cffappslice/app.go:71:12
		task2 := new(struct {
			emitter cff.TaskEmitter
			fn      func(context.Context) error
			ran     cff.AtomicBool
		})
		task2.emitter = cff.NopTaskEmitter()
		task2.fn = func(ctx context.Context) (err error) {
			taskEmitter := task2.emitter
			startTime := time.Now()
			defer func() {
				if task2.ran.Load() {
					taskEmitter.TaskDone(ctx, time.Since(startTime))
				}
			}()

			defer func() {
				recovered := recover()
				if recovered != nil {
					taskEmitter.TaskPanic(ctx, recovered)
					err = &cff.PanicError{
						Value:      recovered,
						Stacktrace: debug.Stack(),
					}
				}
			}()

			defer task2.ran.Store(true)

			err = _71_12(ctx)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return
			}
			taskEmitter.TaskSuccess(ctx)
			return
		}

		sched.Enqueue(ctx, cff.Job{
			Run: task2.fn,
		})
		tasks = append(tasks, task2)

		// github.com/uber-research/rpcshield/go/exampleapps/cffappslice/app.go:63:3
		sliceTask1Slice := _69_6
		for _, val := range sliceTask1Slice {

			val := val
			sliceTask1 := new(struct {
				emitter cff.TaskEmitter
				fn      func(context.Context) error
				ran     cff.AtomicBool
			})
			sliceTask1.fn = func(ctx context.Context) (err error) {
				defer func() {
					recovered := recover()
					if recovered != nil {
						err = &cff.PanicError{
							Value:      recovered,
							Stacktrace: debug.Stack(),
						}
					}
				}()
				err = _63_13(ctx, val)
				return
			}
			sched.Enqueue(ctx, cff.Job{
				Run: sliceTask1.fn,
			})
		}

		if err := sched.Wait(ctx); err != nil {
			parallelEmitter.ParallelError(ctx, err)
			return err
		}
		parallelEmitter.ParallelSuccess(ctx)
		return nil /*line app.go:74*/
	}()

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
