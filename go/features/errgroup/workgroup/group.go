// Package workgroup is a minimal test fixture modeling a common production errgroup-wrapper
// shape (a Group with Go()/Wait()) that rpcshield's errgroup enricher discovers structurally,
// independent of this package's import path.
package workgroup

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// F is the type of function passed to Group.Go.
type F = func(ctx context.Context) error

// Group is a zero-value-usable errgroup wrapper whose work functions receive a context.
type Group struct {
	eg  errgroup.Group
	Ctx context.Context
}

// Go runs f in a new goroutine, tracked by the group.
func (g *Group) Go(f F) {
	ctx := g.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	g.eg.Go(func() error { return f(ctx) })
}

// Wait blocks until all goroutines started via Go have completed, then returns
// the first non-nil error (if any).
func (g *Group) Wait() error {
	return g.eg.Wait()
}
