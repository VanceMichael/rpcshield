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
	err := func() (err error) {

		_43_18 := ctx

		_44_19 := cff.NopEmitter()

		_45_22 := "lucky_flow"

		_46_14 := name

		_46_20 := genCallInput

		_47_15 := &resp

		_47_22 := &int8Result

		_48_12 := func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}

		_51_12 := func(ctx context.Context, lucky int32) (bool, error) {
			return false, nil
		}

		_54_12 := func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Response, error) {
			return &Response{Success: won}, nil
		}

		_57_12 := generalCall

		_58_12 := bm.boundMethod

		_59_12 := iface.DoSomething

		_60_12 := func(ctx context.Context, _ *Response) error {
			return nil
		}
		ctx := _43_18
		var v1 string = _46_14
		var v2 float64 = _46_20
		emitter := cff.EmitterStack(_44_19)

		var (
			flowInfo = &cff.FlowInfo{
				Name:   _45_22,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go",
				Line:   43,
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

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:48:12
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

			v3 = _48_12(ctx, v1)

			taskEmitter.TaskSuccess(ctx)

			return
		}

		task0.job = sched.Enqueue(ctx, cff.Job{
			Run: task0.run,
		})
		tasks = append(tasks, task0)

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:57:12
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
					taskEmitter.TaskPanic(ctx, recovered)
					err = &cff.PanicError{
						Value:      recovered,
						Stacktrace: debug.Stack(),
					}
				}
			}()

			defer task3.ran.Store(true)

			v4, err = _57_12(ctx, v2)

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

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:51:12
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
					taskEmitter.TaskPanic(ctx, recovered)
					err = &cff.PanicError{
						Value:      recovered,
						Stacktrace: debug.Stack(),
					}
				}
			}()

			defer task1.ran.Store(true)

			v5, err = _51_12(ctx, v4)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
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

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:54:12
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

			v6, err = _54_12(ctx, v4, v3, v5)

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

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:58:12
		var (
			v7 int16
		)
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

			v7, err = _58_12(ctx, v4)

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

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:59:12
		var (
			v8 int8
		)
		task5 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task5.emitter = cff.NopTaskEmitter()
		task5.run = func(ctx context.Context) (err error) {
			taskEmitter := task5.emitter
			startTime := time.Now()
			defer func() {
				if task5.ran.Load() {
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

			defer task5.ran.Store(true)

			v8, err = _59_12(ctx, v7)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task5.job = sched.Enqueue(ctx, cff.Job{
			Run: task5.run,
			Dependencies: []*cff.ScheduledJob{
				task4.job,
			},
		})
		tasks = append(tasks, task5)

		// github.com/uber-research/rpcshield/go/exampleapps/cffapp/app.go:60:12
		task6 := new(struct {
			emitter cff.TaskEmitter
			ran     cff.AtomicBool
			run     func(context.Context) error
			job     *cff.ScheduledJob
		})
		task6.emitter = cff.NopTaskEmitter()
		task6.run = func(ctx context.Context) (err error) {
			taskEmitter := task6.emitter
			startTime := time.Now()
			defer func() {
				if task6.ran.Load() {
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

			defer task6.ran.Store(true)

			err = _60_12(ctx, v6)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task6.job = sched.Enqueue(ctx, cff.Job{
			Run: task6.run,
			Dependencies: []*cff.ScheduledJob{
				task2.job,
			},
		})
		tasks = append(tasks, task6)

		if err := sched.Wait(ctx); err != nil {
			flowEmitter.FlowError(ctx, err)
			return err
		}

		*(_47_15) = v6 // *github.com/uber-research/rpcshield/go/exampleapps/cffapp.Response
		*(_47_22) = v8 // int8

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
