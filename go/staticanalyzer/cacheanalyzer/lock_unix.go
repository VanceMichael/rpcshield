//go:build !windows

package cacheanalyzer

import (
	"os"
	"syscall"
)

// tryLockFile non-blockingly takes an exclusive advisory flock. The kernel releases the lock
// automatically when the process exits or the fd closes, so a crashed peer never wedges the cache.
func tryLockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the advisory flock.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// syncDirBestEffort fsyncs the directory entry so a crash preserves the preceding rename. Errors
// are intentionally ignored: the atomic rename itself is already durable-enough for cache reuse.
func syncDirBestEffort(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}
