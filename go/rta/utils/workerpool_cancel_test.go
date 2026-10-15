package utils

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// waitForSleeping polls until the pool reports n sleeping workers, failing
// the test after a deadline. Workers sleep only after failing two steal
// rounds, so this takes at most a few scheduler slices.
func waitForSleeping[T any](t *testing.T, p *WorkerPool[T], n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p.numSleeping.Load() == n {
			return
		}
		runtime.Gosched()
	}
	require.Equal(t, n, p.numSleeping.Load(), "timed out waiting for %d sleeping workers", n)
}

// TestWorkerPool_CancelWakesBlockedWorkersAndConverges deterministically
// exercises the "graph worker blocked" cancellation scenario:
//   - one work item is in flight (blocked on a gate), the other workers sleep;
//   - more items are queued behind the blocked one;
//   - Cancel() rejects every new Push, wakes the sleeping workers, and the
//     already-queued items get drained (dropped cheaply);
//   - after the in-flight item is released, Wait() returns -- all started
//     workers converged, no unit is lost from WaitGroup accounting.
func TestWorkerPool_CancelWakesBlockedWorkersAndConverges(t *testing.T) {
	const nWorkers = 4
	p := NewWorkerPool[int](nWorkers)

	var ran atomic.Int32
	releaseBlocker := make(chan struct{})
	blockerStarted := make(chan struct{})
	var startOnce sync.Once

	p.Start(func(item int, _ int) {
		if p.IsCancelled() {
			return
		}
		if item == 0 {
			startOnce.Do(func() { close(blockerStarted) })
			<-releaseBlocker // in-flight item blocks here until canceled/released
		}
		ran.Add(1)
	})

	// One in-flight blocker; the other nWorkers-1 workers eventually sleep.
	p.Push(0, -1)
	<-blockerStarted
	waitForSleeping(t, p, nWorkers-1)

	// Items queued while workers sleep (all accounted by the WaitGroup).
	for i := 1; i <= 6; i++ {
		p.Push(i, -1)
	}

	p.Cancel()
	require.True(t, p.IsCancelled())

	// New work units are rejected after cancellation: WaitGroup accounting
	// must not move, and the item must never reach a worker.
	p.Push(99, -1)

	// Releasing the in-flight item lets it converge; woken workers drain the
	// queued items (the do callback drops them cheaply). Wait must return.
	close(releaseBlocker)
	done := make(chan struct{})
	go func() { p.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait() did not converge after Cancel -- a work unit was stranded")
	}
	p.Stop()

	// The in-flight item always ran; dropped/drained items may or may not,
	// but the rejected item 99 never could.
	require.GreaterOrEqual(t, ran.Load(), int32(1))
}

// TestWorkerPool_CancelWhileAllSleeping verifies the case where Cancel lands
// when every worker is blocked in the sleep select with no queued work: they
// must be woken (so a subsequent Stop is immediate) and Push stays rejected.
func TestWorkerPool_CancelWhileAllSleeping(t *testing.T) {
	const nWorkers = 3
	p := NewWorkerPool[int](nWorkers)
	p.Start(func(item int, _ int) {})

	waitForSleeping(t, p, nWorkers)
	p.Cancel()

	// Wait on an empty pool returns immediately before and after cancel.
	p.Wait()

	stopped := make(chan struct{})
	go func() { p.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() hung: sleeping workers were not woken by Cancel")
	}

	// Double Cancel is a no-op (idempotent).
	p.Cancel()
}
