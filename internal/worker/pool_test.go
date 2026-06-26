// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package worker // import "miniflux.app/v2/internal/worker"

import (
	"sync"
	"testing"
	"time"

	"miniflux.app/v2/internal/model"
)

// newTestPool builds a pool without starting real workers so we can exercise
// the Push/Shutdown coordination in isolation.
func newTestPool(workers int) *Pool {
	p := &Pool{
		queue: make(chan model.Job),
		done:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-p.done:
					return
				case <-p.queue:
				}
			}
		}()
	}
	return p
}

// TestPushDoesNotPanicWhenShuttingDown guards against the regression where the
// detached `go pool.Push(jobs)` from a manual "refresh all feeds" panicked with
// "send on closed channel" if the process shut down mid-enqueue.
func TestPushDoesNotPanicWhenShuttingDown(t *testing.T) {
	p := newTestPool(0) // no consumers: Push will block on the unbuffered queue

	jobs := make(model.JobList, 500)

	pushReturned := make(chan struct{})
	go func() {
		defer close(pushReturned)
		p.Push(jobs) // must not panic even though nothing drains the queue
	}()

	// Let Push block on the first send, then shut down concurrently.
	time.Sleep(20 * time.Millisecond)
	p.Shutdown()

	select {
	case <-pushReturned:
		// Push unblocked via the done channel and returned cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("Push did not return after Shutdown (still blocked on the queue)")
	}
}

// TestShutdownIsIdempotent ensures Shutdown can be called more than once
// (e.g. signal handler + defer) without panicking on a double close.
func TestShutdownIsIdempotent(t *testing.T) {
	p := newTestPool(2)

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Shutdown()
		}()
	}
	wg.Wait()
}
