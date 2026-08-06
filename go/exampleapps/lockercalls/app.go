package main

import (
	"context"
	"sync"
)

func successHasLock(l sync.Locker) {
	l.Lock()
}

func successHasUnlock(l sync.Locker) {
	l.Unlock()
}

func failureNoLock() {
	_ = context.Background()
}

func main() {
}
