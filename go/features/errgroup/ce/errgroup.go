// Package errgroup is a minimal test fixture modeling a common production errgroup-wrapper
// shape (a Group with Go()/Wait()/SetLimit()) that rpcshield's errgroup enricher discovers
// structurally, independent of this package's import path.
package errgroup

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

// SetLimit limits the number of active goroutines in this group to n.
func (g *Group) SetLimit(n int) {
	g.eg.SetLimit(n)
}
