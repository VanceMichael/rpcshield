//go:build !cff

package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

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
	err := func() (err error) {

		_25_18 := ctx

		_26_19 := cff.NopEmitter()

		_27_22 := "lucky_flow"

		_28_14 := name

		_28_20 := genCallInput

		_29_15 := &resp

		_30_12 := func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}

		_33_12 := func(ctx context.Context, lucky int32) (bool, error) {
			return false, nil
		}

		_35_23 := true

		_36_12 := func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Response, error) {
			return &Response{Success: won}, nil
		}

		_39_12 := generalCall

		_39_42 := int32(42)

		_40_12 := func(ctx context.Context, _ *Response) error {
			return nil
		}
		ctx := _25_18
		var v1 string = _28_14
		var v2 float64 = _28_20
		emitter := cff.EmitterStack(_26_19)

		var (
			flowInfo = &cff.FlowInfo{
				Name:   _27_22,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go",
				Line:   25,
				Column: 9,
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

		// github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go:30:12
		var (
			v3 stringWrapper
		)
		task0 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task0.emitter = cff.NopTaskEmitter()
		task0.run = func(ctx context.Context) (err error) {
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

			v3 = _30_12(ctx, v1)

			taskEmitter.TaskSuccess(ctx)

			return
		}

		task0.job = sched.Enqueue(ctx, cff.Job{
			Run: task0.run,
		})
		tasks = append(tasks, task0)

		// github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go:39:12
		var (
			v4 int32
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
					taskEmitter.TaskPanicRecovered(ctx, recovered)
					v4, err = _39_42, nil
				}
			}()

			defer task3.ran.Store(true)

			v4, err = _39_12(ctx, v2)

			if err != nil {
				taskEmitter.TaskErrorRecovered(ctx, err)
				v4, err = _39_42, nil
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task3.job = sched.Enqueue(ctx, cff.Job{
			Run: task3.run,
		})
		tasks = append(tasks, task3)

		// github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go:33:12
		var (
			v5 bool
		)
		task1 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task1.emitter = cff.NopTaskEmitter()
		task1.run = func(ctx context.Context) (err error) {
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
					taskEmitter.TaskPanicRecovered(ctx, recovered)
					v5, err = _35_23, nil
				}
			}()

			defer task1.ran.Store(true)

			v5, err = _33_12(ctx, v4)

			if err != nil {
				taskEmitter.TaskErrorRecovered(ctx, err)
				v5, err = _35_23, nil
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task1.job = sched.Enqueue(ctx, cff.Job{
			Run: task1.run,
			Dependencies: []*cff.ScheduledJob{
				task3.job,
			},
		})
		tasks = append(tasks, task1)

		// github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go:36:12
		var (
			v6 *Response
		)
		task2 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task2.emitter = cff.NopTaskEmitter()
		task2.run = func(ctx context.Context) (err error) {
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

			v6, err = _36_12(ctx, v4, v3, v5)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task2.job = sched.Enqueue(ctx, cff.Job{
			Run: task2.run,
			Dependencies: []*cff.ScheduledJob{
				task3.job,
				task0.job,
				task1.job,
			},
		})
		tasks = append(tasks, task2)

		// github.com/uber-research/rpcshield/go/exampleapps/cfffallback/app.go:40:12
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

			err = _40_12(ctx, v6)

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
				task2.job,
			},
		})
		tasks = append(tasks, task4)

		if err := sched.Wait(ctx); err != nil {
			flowEmitter.FlowError(ctx, err)
			return err
		}

		*(_29_15) = v6 // *github.com/uber-research/rpcshield/go/exampleapps/cfffallback.Response

		flowEmitter.FlowSuccess(ctx)
		return nil
	}()
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
