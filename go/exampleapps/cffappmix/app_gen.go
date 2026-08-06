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
	err := func() (err error) {

		_54_3 := ctx

		_55_19 := cff.NopEmitter()

		_56_26 := "lucky"

		_57_23 := false

		_58_19 := 2

		_59_12 := func(ctx context.Context) error {
			sw := stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
			fmt.Println(sw)
			return nil
		}

		_64_12 := func(ctx context.Context) error {
			ojHasOrderedBefore, ojErr := HasOrderedBefore(ctx, int8Arg)
			if ojHasOrderedBefore {
				return nil
			}
			return ojErr
		}

		_71_12 := func(ctx context.Context) error {
			resp = &Response{Success: false}
			return errors.New("error")
		}
		ctx := _54_3
		emitter := cff.EmitterStack(_55_19)

		var (
			parallelInfo = &cff.ParallelInfo{
				Name:   _56_26,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go",
				Line:   53,
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
				Concurrency: _58_19, Emitter: schedEmitter,
				ContinueOnError: _57_23,
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

		// github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go:59:12
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

			err = _59_12(ctx)

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

		// github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go:64:12
		task1 := new(struct {
			emitter cff.TaskEmitter
			fn      func(context.Context) error
			ran     cff.AtomicBool
		})
		task1.emitter = cff.NopTaskEmitter()
		task1.fn = func(ctx context.Context) (err error) {
			taskEmitter := task1.emitter
			startTime := time.Now()
			defer func() {
				if task1.ran.Load() {
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

			defer task1.ran.Store(true)

			err = _64_12(ctx)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return
			}
			taskEmitter.TaskSuccess(ctx)
			return
		}

		sched.Enqueue(ctx, cff.Job{
			Run: task1.fn,
		})
		tasks = append(tasks, task1)

		// github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go:71:12
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

		if err := sched.Wait(ctx); err != nil {
			parallelEmitter.ParallelError(ctx, err)
			return err
		}
		parallelEmitter.ParallelSuccess(ctx)
		return nil /*line app.go:74*/
	}()

	err1 := func() (err error) {

		_77_19 := ctx

		_78_14 := int8Arg

		_79_19 := cff.NopEmitter()

		_80_22 := "lucky_flow"

		_81_12 := func(ctx context.Context, lucky int8) (*Response, error) {
			return &Response{Success: false}, nil
		}

		_84_12 := func(ctx context.Context, _ *Response) error {
			return nil
		}
		ctx := _77_19
		var v1 int8 = _78_14
		emitter := cff.EmitterStack(_79_19)

		var (
			flowInfo = &cff.FlowInfo{
				Name:   _80_22,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go",
				Line:   77,
				Column: 10,
			}
			flowEmitter = emitter.FlowInit(flowInfo)

			schedInfo = &cff.SchedulerInfo{
				Name:      flowInfo.Name,
				Directive: cff.FlowDirective,
				File:      flowInfo.File,
				Line:      flowInfo.Line,
				Column:    flowInfo.Column,
			}

			// possibly unused
			_ = flowInfo
		)

		startTime := time.Now()
		defer func() { flowEmitter.FlowDone(ctx, time.Since(startTime)) }()

		schedEmitter := emitter.SchedulerInit(schedInfo)

		sched := cff.NewScheduler(
			cff.SchedulerParams{
				Emitter: schedEmitter,
			},
		)

		var tasks []*struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		}
		defer func() {
			for _, t := range tasks {
				if !t.ran.Load() {
					t.emitter.TaskSkipped(ctx, err)
				}
			}
		}()

		// github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go:81:12
		var (
			v2 *Response
		)
		task3 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task3.emitter = cff.NopTaskEmitter()
		task3.run = func(ctx context.Context) (err error) {
			taskEmitter := task3.emitter
			startTime := time.Now()
			defer func() {
				if task3.ran.Load() {
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

			defer task3.ran.Store(true)

			v2, err = _81_12(ctx, v1)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task3.job = sched.Enqueue(ctx, cff.Job{
			Run: task3.run,
		})
		tasks = append(tasks, task3)

		// github.com/uber-research/rpcshield/go/exampleapps/cffappmix/app.go:84:12
		task4 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task4.emitter = cff.NopTaskEmitter()
		task4.run = func(ctx context.Context) (err error) {
			taskEmitter := task4.emitter
			startTime := time.Now()
			defer func() {
				if task4.ran.Load() {
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

			defer task4.ran.Store(true)

			err = _84_12(ctx, v2)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task4.job = sched.Enqueue(ctx, cff.Job{
			Run: task4.run,
			Dependencies: []*cff.ScheduledJob{
				task3.job,
			},
		})
		tasks = append(tasks, task4)

		if err := sched.Wait(ctx); err != nil {
			flowEmitter.FlowError(ctx, err)
			return err
		}

		flowEmitter.FlowSuccess(ctx)
		return nil
	}()

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
