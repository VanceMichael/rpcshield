//go:build !cff

// Package controller implements the rpcshielddemo business logic.
package controller

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"

	"go.uber.org/cff"
	"golang.org/x/sync/errgroup"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/httpservicefoo"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/lottery"
)

// Result is the demo's result entity.
type Result struct {
	Name string
	Won  bool
}

// Controller orchestrates the business logic between the lottery gateway and the demo's storage.
type Controller interface {
	Create(context.Context, string) (*Result, error)
	Get(context.Context, string) (*Result, error)
}

type controller struct {
	lottery        lottery.Gateway
	httpservicefoo httpservicefoo.Gateway
}

// New constructs a new demo Controller.
func New(l lottery.Gateway, h httpservicefoo.Gateway) Controller {
	return &controller{lottery: l, httpservicefoo: h}
}

type stringWrapper struct {
	name string
}

// Create synchronously creates a new Result, fanning out via cff.
func (c *controller) Create(ctx context.Context, name string) (*Result, error) {
	random := rand.Int31()
	var result *Result
	err := func() (err error) {

		_48_18 := ctx

		_49_19 := cff.NopEmitter()

		_50_22 := "lucky_flow"

		_51_14 := random

		_51_22 := name

		_52_15 := &result

		_53_12 := func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}

		_56_12 := func(ctx context.Context, lucky int32) (bool, error) {
			return c.lottery.Play(ctx, lucky)
		}

		_59_12 := func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Result, error) {
			return &Result{Name: prefixName.name, Won: won}, nil
		}

		_62_12 := func(ctx context.Context, result *Result) error {
			return nil
		}
		ctx := _48_18
		var v1 int32 = _51_14
		var v2 string = _51_22
		emitter := cff.EmitterStack(_49_19)

		var (
			flowInfo = &cff.FlowInfo{
				Name:   _50_22,
				File:   "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller/controller.go",
				Line:   48,
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

		// github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller/controller.go:53:12
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

			v3 = _53_12(ctx, v2)

			taskEmitter.TaskSuccess(ctx)

			return
		}

		task0.job = sched.Enqueue(ctx, cff.Job{
			Run: task0.run,
		})
		tasks = append(tasks, task0)

		// github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller/controller.go:56:12
		var (
			v4 bool
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

			v4, err = _56_12(ctx, v1)

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
		})
		tasks = append(tasks, task1)

		// github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller/controller.go:59:12
		var (
			v5 *Result
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

			v5, err = _59_12(ctx, v1, v3, v4)

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
				task0.job,
				task1.job,
			},
		})
		tasks = append(tasks, task2)

		// github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller/controller.go:62:12
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

			err = _62_12(ctx, v5)

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
			Dependencies: []*cff.ScheduledJob{
				task2.job,
			},
		})
		tasks = append(tasks, task3)

		if err := sched.Wait(ctx); err != nil {
			flowEmitter.FlowError(ctx, err)
			return err
		}

		*(_52_15) = v5 // *github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller.Result

		flowEmitter.FlowSuccess(ctx)
		return nil
	}()
	return result, err
}

const _testrpcshield = true

// Get synchronously retrieves a previously-created Result.
func (c *controller) Get(ctx context.Context, id string) (*Result, error) {
	if _testrpcshield {
		// A type cast is also a call expression AST node, this tests rpcshield's ability to handle type casts.
		_ = context.Context(ctx)
		x := rand.Int31() % 10
		switch x {
		case 0:
			return c.failCloseDemo(ctx, id)
		case 1:
			return c.failOpenDemo(ctx, id)
		case 2:
			return c.failCloseAnnotationDemo(ctx, id)
		case 3:
			return c.failOpenAnnotationDemo(ctx, id)
		case 4:
			return c.suppressFailCloseAnnotationDemo(ctx, id)
		case 5: // For testing rpcshield predefined analyzer that handles $thunks/$bounds.
			b := c.boundExample
			return b(ctx, id)
		case 6:
			// As of now, it is not detectable by rpcshield. But we track all outbounds
			// that are reachable from "go" statements.
			go func() {
				c.failCloseDemo(ctx, id)
			}()
		case 7:
			return c.errGroupDemo(ctx, id)
		case 8:
			return c.anonymousFuncDemo(ctx, id)
		case 9:
			return c.httpservicefooDemo(ctx, id)
		}
	}
	return &Result{}, nil
}

func (c *controller) failCloseDemo(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		return nil, err
	}
	return &Result{Name: "failCloseDemo", Won: won}, nil
}

func (c *controller) failOpenDemo(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		return nil, nil
	}
	return &Result{Name: "failOpenDemo", Won: won}, nil
}

func (c *controller) boundExample(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		return nil, nil
	}
	return &Result{Name: "boundExample", Won: won}, nil
}

func (c *controller) httpservicefooDemo(ctx context.Context, id string) (*Result, error) {
	_, err := c.httpservicefoo.Call(ctx, "test message")
	if err != nil {
		return nil, err
	}
	return &Result{Name: "httpservicefooDemo", Won: true}, nil
}

// rpcshield:failclose
func (c *controller) failCloseAnnotationDemo(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		res2, err2 := &Result{Name: "failCloseDemo", Won: won}, error(nil)
		if err2 != nil {
			return res2, err2
		}
		return nil, err
	}
	return &Result{Name: "failCloseDemo", Won: won}, nil
}

// rpcshield:failopen
func (c *controller) failOpenAnnotationDemo(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		res2, err2 := &Result{Name: "failCloseDemo", Won: won}, error(nil)
		if err2 != nil {
			return res2, err2
		}
		return nil, err
	}
	return &Result{Name: "failCloseDemo", Won: won}, nil
}

// rpcshield:suppress-fail-close
func (c *controller) suppressFailCloseAnnotationDemo(ctx context.Context, id string) (*Result, error) {
	won, err := c.lottery.Play(ctx, 42)
	if err != nil {
		res2, err2 := &Result{Name: "failCloseDemo", Won: won}, error(nil)
		if err2 != nil {
			return res2, err2
		}
		return nil, err
	}
	return &Result{Name: "failCloseDemo", Won: won}, nil
}

func (c *controller) errGroupDemo(ctx context.Context, id string) (*Result, error) {
	var eg errgroup.Group
	eg.Go(func() error {
		_, err := c.lottery.Play(ctx, 42)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return &Result{Name: "demo", Won: true}, nil
}

func (c *controller) anonymousFuncDemo(ctx context.Context, id string) (*Result, error) {
	f := func(ctx context.Context, id string) error {
		_, err := c.lottery.Play(ctx, 42)
		return err
	}
	if err := f(ctx, id); err != nil {
		return nil, err
	}
	return &Result{Name: "demo", Won: true}, nil
}
