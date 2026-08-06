// Package safegroup is a minimal test fixture modeling a common production errgroup-wrapper
// shape (a Group with Go()/Wait()) that rpcshield's errgroup enricher discovers structurally,
// independent of this package's import path.
package safegroup

import "golang.org/x/sync/errgroup"

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
