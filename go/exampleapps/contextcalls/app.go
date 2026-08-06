package main

import (
	"context"
	"sync"
)

func failureHasLock(l sync.Locker) {
	l.Lock()
}

func failureHasUnlock(l sync.Locker) {
	l.Unlock()
}

func successDeadline(c context.Context) {
	_, _ = c.Deadline()
}

func successDone(c context.Context) {
	_ = c.Done()
}

func successErr(c context.Context) {
	_ = c.Err()
}

func successValue(c context.Context, key interface{}) {
	_ = c.Value(key)
}

func main() {
}
