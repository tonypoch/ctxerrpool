package main

import (
	"context"
	"log"
	"sync"

	"github.com/MicahParks/ctxerrgroup"
)

func main() {

	// Create a wait group so that the cancellation error gets caught.
	errWg := &sync.WaitGroup{}
	errWg.Add(1)

	// Create an error handler that
	var errorHandler ctxerrgroup.ErrorHandler
	errorHandler = func(group ctxerrgroup.Group, err error) {
		defer errWg.Done()
		log.Printf("An error occurred. Error: %s\nKilling group.\n", err.Error())
		group.Kill()
	}

	// Create a worker group with 1 worker and no buffer for queued work. Do not handle errors in a separate goroutine.
	group := ctxerrgroup.New(1, 0, false, errorHandler)

	// Create a wait group so the work actually starts.
	//
	// Wait groups are typically unnecessary, it just helps make a more complete example.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create some work that respects it's given context. Give it a wait group to decrement so the worker group actually
	// starts the work.
	var work ctxerrgroup.Work
	work = func(ctx context.Context) (err error) {
		wg.Done()

		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Create a context for the work.
	ctx, cancel := context.WithCancel(context.Background())

	// Send the work to the group.
	group.AddWorkItem(ctx, cancel, work)

	// Wait for the work to start.
	wg.Wait()

	// Cancel the work.
	cancel()

	// Wait for the error to be handled.
	errWg.Wait()
}
