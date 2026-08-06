// Package safego is a minimal test fixture reproducing just the Go() call shape rpcshield's
// safego enricher looks for structurally and by import path (see graph.DefaultSafeGoPkgPath).
package safego

// Go is just like the built-in `go` keyword. The real implementation additionally
// recovers goroutine panics; that behavior isn't needed for this fixture.
func Go(f func()) {
	go f()
}
