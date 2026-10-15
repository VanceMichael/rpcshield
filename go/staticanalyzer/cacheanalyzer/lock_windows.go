//go:build windows

package cacheanalyzer

import (
	"os"
)

// On platforms without flock, locking degrades to unlocked coordination (temp-file + atomic
// rename still prevents torn reads). Multi-process write races merge-and-replace: a losing
// rename is retried on the next flush with the winner's records merged in.

func tryLockFile(f *os.File) error  { return nil }
func unlockFile(f *os.File) error  { return nil }
func syncDirBestEffort(dir string) {}
