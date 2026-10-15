//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris

package serviceregistry

import (
	"fmt"
	"os"
	"syscall"
)

// refreshLock is an exclusive, process-scoped advisory file lock (flock) that
// serializes registry refreshes of the same workspace. Readers never take it;
// even without the lock the active-pointer rename alone guarantees that
// readers only ever observe a complete old or new generation -- the lock
// additionally prevents concurrent writers from duplicating the expensive
// Grail fetch/Fx scan and from racing generation pruning.
type refreshLock struct {
	f *os.File
}

// acquireRefreshLock blocks until the exclusive refresh lock at path is held.
// The lock is released automatically when the process exits.
func acquireRefreshLock(path string) (*refreshLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquire exclusive flock: %w", err)
	}
	return &refreshLock{f: f}, nil
}

func (l *refreshLock) release() error {
	// Unlock best-effort; closing the descriptor also releases the flock.
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
