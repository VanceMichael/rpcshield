package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// signalContext returns a context that is canceled on the first SIGINT or
// SIGTERM (Ctrl-C in a terminal, or a container orchestrator's stop signal).
// A second signal restores Go's default handler and re-raises the signal to
// the process, so an impatient user can still force-kill a run that is taking
// too long to converge -- while a single press performs the graceful shutdown
// the rest of the pipeline implements (stop accepting targets/work, wait for
// in-flight workers and LLM requests, preserve trusted artifacts, clean up
// the lifted GOPATH).
//
// The returned stop function detaches the handler and releases the watcher
// goroutine; callers should defer it. Like signal.NotifyContext's stop, it
// also cancels the context, so it must only run after the work has finished.
func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		select {
		case <-sigCh:
			// First signal: graceful cancellation.
			cancel()
		case <-done:
			return
		}
		select {
		case sig := <-sigCh:
			// Second signal: restore default behavior and force-exit.
			signal.Reset(os.Interrupt, syscall.SIGTERM)
			if s, ok := sig.(syscall.Signal); ok {
				_ = syscall.Kill(syscall.Getpid(), s)
			}
		case <-done:
		}
	}()

	stop := sync.OnceFunc(func() {
		close(done)
		signal.Stop(sigCh)
		cancel()
	})
	return ctx, stop
}
