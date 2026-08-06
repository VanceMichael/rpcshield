// Package safeerrgroup is a minimal test fixture modeling a common production errgroup-wrapper
// shape (a SafeErrGroup with Go()/Wait()) that rpcshield's errgroup enricher discovers
// structurally, independent of this package's import path.
package safeerrgroup

import "golang.org/x/sync/errgroup"

// SafeErrGroup is a zero-value-usable errgroup wrapper.
type SafeErrGroup struct {
	eg errgroup.Group
}

// Go runs fn in a new goroutine, tracked by the group.
func (g *SafeErrGroup) Go(fn func() error) {
	g.eg.Go(fn)
}

// Wait blocks until all goroutines started via Go have completed, then returns
// the first non-nil error (if any).
func (g *SafeErrGroup) Wait() error {
	return g.eg.Wait()
}
