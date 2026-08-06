// Package errgroup is a minimal test fixture modeling a common production errgroup-wrapper
// shape (a PanicSafeErrGroup with Go()/Wait()) that rpcshield's errgroup enricher discovers
// structurally, independent of this package's import path.
package errgroup

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

// PanicSafeErrGroup is a zero-value-usable errgroup wrapper. The real implementation also
// recovers goroutine panics; that behavior isn't needed for this fixture.
type PanicSafeErrGroup struct {
	eg errgroup.Group
}

// Go runs f (a func() error or func()) in a new goroutine, tracked by the group.
func (g *PanicSafeErrGroup) Go(f any, args ...any) {
	g.eg.Go(func() error { return callFunction(f, args...) })
}

// Wait blocks until all goroutines started via Go have completed, then returns
// the first non-nil error (if any).
func (g *PanicSafeErrGroup) Wait() error {
	return g.eg.Wait()
}

func callFunction(f any, _ ...any) error {
	switch fn := f.(type) {
	case func() error:
		return fn()
	case func():
		fn()
		return nil
	default:
		return fmt.Errorf("unsupported function type %T", f)
	}
}
