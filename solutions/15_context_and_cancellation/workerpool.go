package ctxflow

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// WorkerPool runs handle over submitted jobs on a fixed set of goroutines.
//
// Submit's context governs admission only. The handler gets the pool's own
// context, which Shutdown cancels once its deadline passes.
type WorkerPool[J any] struct {
	handle func(ctx context.Context, job J) error
	jobs   chan J

	// closing is closed first by Shutdown so blocked Submits give up.
	closing   chan struct{}
	closeOnce sync.Once
	// mu serialises the jobs-channel close against in-flight sends.
	mu     sync.RWMutex
	closed bool

	handlerCtx     context.Context
	cancelHandlers context.CancelCauseFunc
	wg             sync.WaitGroup

	succeeded atomic.Int64
	failed    atomic.Int64
	abandoned atomic.Int64
}

// NewWorkerPool starts workers goroutines that consume a queue of capacity
// queue. workers must be at least 1 and queue at least 0.
func NewWorkerPool[J any](workers, queue int, handle func(ctx context.Context, job J) error) (*WorkerPool[J], error) {
	if workers < 1 || queue < 0 {
		return nil, fmt.Errorf("%w: workers must be >= 1 and queue >= 0", ErrInvalidArgument)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	p := &WorkerPool[J]{
		handle:         handle,
		jobs:           make(chan J, queue),
		closing:        make(chan struct{}),
		handlerCtx:     ctx,
		cancelHandlers: cancel,
	}
	for range workers {
		p.wg.Go(p.work)
	}
	return p, nil
}

// Submit enqueues job, waiting for queue space. It returns context.Cause(ctx)
// if ctx ends first, and ErrClosed once Shutdown has begun.
func (p *WorkerPool[J]) Submit(ctx context.Context, job J) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.jobs <- job:
		return nil
	case <-p.closing:
		return ErrClosed
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// Shutdown stops admission and waits for the queue to drain and in-flight
// handlers to return. If ctx ends first, the handler context is cancelled,
// queued jobs that have not started are counted as abandoned, and Shutdown
// still waits for in-flight handlers before returning context.Cause(ctx).
// The first return value is the number of abandoned jobs. A second Shutdown
// returns (0, ErrClosed).
func (p *WorkerPool[J]) Shutdown(ctx context.Context) (int, error) {
	first := false
	p.closeOnce.Do(func() {
		first = true
		close(p.closing)
	})
	if !first {
		return 0, ErrClosed
	}

	p.mu.Lock()
	p.closed = true
	close(p.jobs)
	p.mu.Unlock()

	finished := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(finished)
	}()

	var err error
	select {
	case <-finished:
	case <-ctx.Done():
		err = context.Cause(ctx)
		p.cancelHandlers(err)
		<-finished
	}
	p.cancelHandlers(nil)
	return int(p.abandoned.Load()), err
}

// Succeeded reports how many jobs the handler completed without error.
func (p *WorkerPool[J]) Succeeded() int { return int(p.succeeded.Load()) }

// Failed reports how many jobs the handler returned an error for, including
// jobs cancelled mid-flight.
func (p *WorkerPool[J]) Failed() int { return int(p.failed.Load()) }

func (p *WorkerPool[J]) work() {
	for job := range p.jobs {
		if p.handlerCtx.Err() != nil {
			p.abandoned.Add(1)
			continue
		}
		if err := p.handle(p.handlerCtx, job); err != nil {
			p.failed.Add(1)
		} else {
			p.succeeded.Add(1)
		}
	}
}
