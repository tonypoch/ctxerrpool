package ctxerrgroup

import (
	"context"
	"errors"
	"sync"
)

var (

	// ErrCantDo indicates that there was a failure to send the function to work on to a worker before the context
	// expired.
	ErrCantDo = errors.New("failed to send work item to a worker before the context expired")
)

// Work is a function that utilizes the given context properly and returns an error.
type Work func(workCtx context.Context) (err error)

// workItem holds a function to work on and the context for it.
type workItem struct {
	cancel      context.CancelFunc
	ctx         context.Context
	decremented bool
	mux         *sync.Mutex
	wg          *sync.WaitGroup
	work        Work
}

// worker consumes work items while from the Group and sends unhandled errors back to the Group error handler.
type worker struct {
	death   chan struct{}
	do      <-chan *workItem
	errChan chan<- error
}

// start is the main loop for a worker.
func (w worker) start() {

	// Wait for a condition in a loop until death.
	for {
		select {

		// If told to die, end the goroutine.
		case <-w.death:
			return

		// If some work was received, do it.
		case work := <-w.do:

			// Consume the work item.
			w.work(*work)

			// The work is finished.
			work.finished()

			// Prevent work from being randomly selected if both cases are ready.
			if dead(w.death) {
				return
			}
		}
	}
}

// work is performed when a worker receives some work to do. If it returns true, the worker died before the work was
// finished.
func (w worker) work(item workItem) {

	// Check to make sure the group didn't die and work case was selected randomly.
	if dead(w.death) {
		return
	}

	// Check to make sure the context is still valid.
	if err := expired(item.ctx); err != nil {
		w.errChan <- err
		return
	}

	// Create a mutex to only allow for one context related error to be reported over the channel.
	muxCtxErr := &sync.Mutex{}

	// Create a pointer to a boolean that indicates if a context error has already been reported over the channel.
	hasCtxErr := &[]bool{false}[0] // *false

	// Create a channel that notifies us when the work has been completed.
	finished := make(chan struct{})

	// AddWorkItem the work asynchronously.
	go w.doWork(item, finished, hasCtxErr, muxCtxErr)

	// Wait for a condition.
	select {

	// The context is over. Report any error on the error channel.
	case <-item.ctx.Done():
		muxCtxErr.Lock()
		if !*hasCtxErr {
			*hasCtxErr = true
			w.errChan <- item.ctx.Err()
		}
		muxCtxErr.Unlock()

	// The worker died before finishing the work.
	case <-w.death:

	// Successfully finished the work.
	case <-finished:
	}

	return
}

// doWork actually performs the work item.
func (w worker) doWork(item workItem, finished chan struct{}, hasCtxErr *bool, muxCtxErr *sync.Mutex) {
	if err := item.work(item.ctx); err != nil {

		// If the error is a context error and hasn't been reported already, report it. If it's not a context error,
		// report it.
		muxCtxErr.Lock()
		if (!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)) || (errors.Is(err, context.Canceled) && !*hasCtxErr || errors.Is(err, context.DeadlineExceeded) && !*hasCtxErr) {
			*hasCtxErr = true
			w.errChan <- err
		}
		muxCtxErr.Unlock()
	}

	// The work is done.
	close(finished)
}
