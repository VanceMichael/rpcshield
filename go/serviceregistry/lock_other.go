//go:build !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris

package serviceregistry

// On platforms without flock(2) refreshes are not cross-process serialized.
// This is safe for correctness: each refresh stages in a unique directory and
// publishes via an atomic rename of the active pointer, so readers still only
// observe a complete old or new generation (and never services from one writer
// paired with another writer's mapping). Concurrent refreshes may waste work
// and compete on pruning, which is benign.
type refreshLock struct{}

func acquireRefreshLock(string) (*refreshLock, error) {
	return &refreshLock{}, nil
}

func (l *refreshLock) release() error { return nil }
