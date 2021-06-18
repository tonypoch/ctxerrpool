package main

import (
	"context"
	"log"
	"sync"

	"github.com/MicahParks/ctxerrpool"
)

func main() {

	// Create a wait pool so that the cancellation error gets caught.
	errWg := &sync.WaitGroup{}
	errWg.Add(1)

	// Create an error handler that
	var errorHandler ctxerrpool.ErrorHandler
	errorHandler = func(pool ctxerrpool.Pool, err error) {
		defer errWg.Done()
		log.Printf("An error occurred. Error: %s\nKilling pool.\n", err.Error())
		pool.Kill()
	}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, errorHandler)

	// Create a wait pool so the work actually starts.
	//
	// Wait pools are typically unnecessary, it just helps make a more complete example.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create some work that respects it's given context. Give it a wait pool to decrement so the worker pool actually
	// starts the work.
	var work ctxerrpool.Work
	work = func(ctx context.Context) (err error) {
		wg.Done()

		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Create a context for the work.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send the work to the pool.
	pool.AddWorkItem(ctx, work)

	// Wait for the work to start.
	wg.Wait()

	// Cancel the work.
	cancel()

	// Wait for the error to be handled.
	errWg.Wait()
}
