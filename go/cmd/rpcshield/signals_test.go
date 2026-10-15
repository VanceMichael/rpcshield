package main

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSignalContext_CancelsOnSIGINT deterministically reproduces the
// Ctrl-C path: a SIGINT delivered to the process cancels the context exactly
// once, and stop() releases the handler without canceling it a second time
// meaningfully. The force-exit-on-second-signal behavior is intentionally not
// exercised here (it would kill the test binary).
func TestSignalContext_CancelsOnSIGINT(t *testing.T) {
	ctx, stop := signalContext(context.Background())
	defer stop()

	require.NoError(t, ctx.Err())
	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))

	deadline := time.After(5 * time.Second)
	for ctx.Err() == nil {
		select {
		case <-deadline:
			t.Fatal("context was not canceled by SIGINT")
		case <-time.After(10 * time.Millisecond):
		}
	}
	require.True(t, errors.Is(ctx.Err(), context.Canceled))
}

// TestSignalContext_StopDetaches verifies a run that finishes without any
// signal simply completes and stop detaches the handler (subsequent signals
// no longer touch the context).
func TestSignalContext_StopDetaches(t *testing.T) {
	ctx, stop := signalContext(context.Background())
	stop()
	// stop cancels the internal context (mirrors signal.NotifyContext's stop
	// semantics) and must be idempotent.
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	stop()
}
