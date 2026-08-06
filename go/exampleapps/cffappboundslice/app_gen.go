//go:build !cff

package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

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
	err := func() (err error) {

		_39_18 := ctx

		_40_19 := cff.NopEmitter()

		_41_22 := "lucky_flow"

		_42_14 := name

		_43_15 := &resp

		_44_12 := func(ctx context.Context, name string) (*Response, error) {
			for _, validator := range validators {
				if err := validator(ctx, int32(len(name))); err != nil {
					return nil, err
				}
			}
			return &Response{Success: true}, nil
		}
		ctx := _39_18
		var v1 string = _42_14
		emitter := cff.EmitterStack(_40_19)

		var (
			flowInfo = &cff.FlowInfo{
				Name:   _41_22,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/cffappboundslice/app.go",
				Line:   39,
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

		// github.com/uber-research/rpcshield/go/exampleapps/cffappboundslice/app.go:44:12
		var (
			v2 *Response
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

			v2, err = _44_12(ctx, v1)

			if err != nil {
				taskEmitter.TaskError(ctx, err)
				return err
			} else {
				taskEmitter.TaskSuccess(ctx)
			}

			return
		}

		task0.job = sched.Enqueue(ctx, cff.Job{
			Run: task0.run,
		})
		tasks = append(tasks, task0)

		if err := sched.Wait(ctx); err != nil {
			flowEmitter.FlowError(ctx, err)
			return err
		}

		*(_43_15) = v2 // *github.com/uber-research/rpcshield/go/exampleapps/cffappboundslice.Response

		flowEmitter.FlowSuccess(ctx)
		return nil
	}()
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
