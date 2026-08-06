// Package util is a minimal test fixture modeling a Group shape (including its WaitContext/
// AllErrors methods, which are NOT in rpcshield's errgroup enricher's allowed method set, so
// this type is intentionally NOT detected as errgroup-like) independent of import path.
package util

import (
	"context"

	"golang.org/x/sync/errgroup"
)

// Group is a zero-value-usable errgroup wrapper.
type Group struct {
	eg errgroup.Group
}

// Go runs f in a new goroutine, tracked by the group.
func (g *Group) Go(f func() error) {
	g.eg.Go(f)
}

// Wait blocks until all goroutines started via Go have completed, then returns
// the first non-nil error (if any).
func (g *Group) Wait() error {
	return g.eg.Wait()
}

// WaitContext waits like Wait, but also returns early if ctx is cancelled.
func (g *Group) WaitContext(ctx context.Context) error {
	done := make(chan error, 1)
	go func() { done <- g.eg.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Limit limits the number of active goroutines in this group to limit.
func (g *Group) Limit(limit int) {
	g.eg.SetLimit(limit)
}

// AllErrors reports the number of goroutines that returned an error, and the first such error.
func (g *Group) AllErrors() (int, error) {
	err := g.eg.Wait()
	if err != nil {
		return 1, err
	}
	return 0, nil
}
