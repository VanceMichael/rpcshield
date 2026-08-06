//go:build cff

// Package controller implements the rpcshielddemo business logic.
package controller

import (
	"context"
	"fmt"
	"math/rand"

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
	err := cff.Flow(ctx,
		cff.WithEmitter(cff.NopEmitter()),
		cff.InstrumentFlow("lucky_flow"),
		cff.Params(random, name),
		cff.Results(&result),
		cff.Task(func(ctx context.Context, name string) stringWrapper {
			return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		}),
		cff.Task(func(ctx context.Context, lucky int32) (bool, error) {
			return c.lottery.Play(ctx, lucky)
		}),
		cff.Task(func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*Result, error) {
			return &Result{Name: prefixName.name, Won: won}, nil
		}),
		cff.Task(func(ctx context.Context, result *Result) error {
			return nil
		},
			cff.Invoke(true),
		),
	)
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
