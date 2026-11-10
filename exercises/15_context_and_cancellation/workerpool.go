package ctxflow

import "context"

// WorkerPool runs handle for submitted jobs on a fixed set of workers.
type WorkerPool[J any] struct{}

// NewWorkerPool starts workers goroutines consuming a queue of capacity queue.
// workers < 1 or queue < 0 wraps ErrInvalidArgument.
func NewWorkerPool[J any](workers, queue int, handle func(ctx context.Context, job J) error) (*WorkerPool[J], error) {
	return nil, nil
}

// Submit enqueues job. It blocks while the queue is full. If ctx is done first it
// returns context.Cause(ctx) and the job is not enqueued. After Shutdown has
// begun, Submit returns ErrClosed. A nil return means the job is either handled
// or counted in Shutdown's abandoned result. The submitter's ctx governs admission
// only; it does not cancel the handler.
func (p *WorkerPool[J]) Submit(ctx context.Context, job J) error {
	return nil
}

// Shutdown stops admission and drains: queued jobs keep running until the queue
// is empty and every worker has returned. If ctx ends first, Shutdown abandons:
// the handler context is cancelled, jobs not yet started are dropped and counted
// in abandoned, and Shutdown still waits for in-flight handlers to return. The
// error is nil for a clean drain and context.Cause(ctx) when abandoning. A second
// call returns (0, ErrClosed).
func (p *WorkerPool[J]) Shutdown(ctx context.Context) (abandoned int, err error) {
	return 0, nil
}

// Succeeded counts jobs whose handler returned nil.
func (p *WorkerPool[J]) Succeeded() int { return 0 }

// Failed counts jobs whose handler returned an error, including handlers that
// returned because they were cancelled by an abandoning Shutdown.
func (p *WorkerPool[J]) Failed() int { return 0 }
