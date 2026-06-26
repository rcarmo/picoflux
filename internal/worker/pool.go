// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package worker // import "miniflux.app/v2/internal/worker"

import (
	"sync"

	"miniflux.app/v2/internal/model"
	"miniflux.app/v2/internal/storage"
)

// Pool manages a set of background workers that process feed refresh jobs.
type Pool struct {
	queue    chan model.Job
	done     chan struct{}
	wg       sync.WaitGroup
	shutdown sync.Once
}

// Push sends a list of jobs to the queue.
//
// Push is safe to call from a detached goroutine: if the pool is shut down
// while jobs are still being enqueued, it stops sending instead of panicking on
// a closed channel. (The queue is never closed; workers stop via the done
// channel.)
func (p *Pool) Push(jobs model.JobList) {
	for _, job := range jobs {
		select {
		case p.queue <- job:
		case <-p.done:
			return
		}
	}
}

// Shutdown signals all workers to stop and waits for in-flight jobs to finish.
// It is idempotent and safe to call concurrently with Push.
func (p *Pool) Shutdown() {
	p.shutdown.Do(func() {
		close(p.done)
	})
	p.wg.Wait()
}

// NewPool creates a pool of background workers.
func NewPool(store *storage.Storage, nbWorkers int) *Pool {
	workerPool := &Pool{
		queue: make(chan model.Job),
		done:  make(chan struct{}),
	}

	for i := range nbWorkers {
		workerPool.wg.Add(1)
		worker := &worker{id: i, store: store}
		go worker.Run(workerPool.queue, workerPool.done, &workerPool.wg)
	}

	return workerPool
}
