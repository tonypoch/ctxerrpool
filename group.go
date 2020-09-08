package ctxerrgroup

import (
	"context"
	"sync"
)

// ErrorHandler is a function that receives an error and handles it.
type ErrorHandler func(group Group, err error)

// Group is the way to control a group of worker goroutines that understand context.Context and error handling.
type Group struct {
	death   chan struct{}
	do      chan<- *workItem
	errChan chan error
	wg      *sync.WaitGroup
}

// New creates a new Group.
func New(workers uint, workItemBuffer uint, asyncErrors bool, errorHandler ErrorHandler) Group {

	// Create the required channels and wait group.
	death := make(chan struct{})
	do := make(chan *workItem, workItemBuffer)
	errChan := make(chan error)
	wg := &sync.WaitGroup{}

	// Make the Group.
	group := Group{
		death:   death,
		do:      do,
		errChan: errChan,
		wg:      wg,
	}

	// Handle all outgoing errors async.
	go group.handleErrors(asyncErrors, errorHandler)

	// Create the desired number of workers and start them.
	for i := uint(0); i < workers; i++ {
		w := worker{
			death:   death,
			do:      do,
			errChan: errChan,
		}
		go w.start()
	}

	return group
}

// Death returns a channel that will close when the Group has died.
func (g Group) Death() <-chan struct{} {
	return g.death
}

// AddWorkItem takes in context information and a Work function and gives it to a worker. This can block if all workers
// are busy and the work item buffer is full. If asyncAdd is true, the function will not block and launch another
// goroutine to send the work item.
func (g Group) AddWorkItem(ctx context.Context, cancel context.CancelFunc, asyncAdd bool, work Work) {

	// Check to make sure the group isn't dead on arrival.
	if g.Dead() {
		return
	}

	// Increment the wait group.
	g.wg.Add(1)

	// Create the work item.
	item := &workItem{
		cancel: cancel,
		ctx:    ctx,
		mux:    &sync.Mutex{},
		wg:     g.wg,
		work:   work,
	}

	if asyncAdd {
		go g.sendWorkItem(ctx, item)
	} else {
		g.sendWorkItem(ctx, item) // This will block if no worker is ready and the work item buffer is full.
	}
}

// Dead determines if the group is dead.
func (g Group) Dead() bool {
	return dead(g.death)
}

// Done mimics the functionality of the context.Context Done method. It returns a channel that will close when all
// given work has been completed or when the group dies.
func (g Group) Done() <-chan struct{} {

	// Make a channel to close.
	c := make(chan struct{})

	// Launch a goroutine that will close the channel when all work has been completed or the group dies.
	go g.mimic(c)

	return c
}

// Kill tells all the worker goroutines and work items to end.
func (g Group) Kill() {
	close(g.death)
}

// Wait mimics the functionality of the sync.WaitGroup Wait method. It returns when all given work has been completed or
// when the group dies.
func (g Group) Wait() {
	g.mimic(nil)
}

// handleErrors is meant to be a goroutine that will handle all errors returned from work items. It takes in an error
// handler function and an async boolean. If the async boolean is true, all errors returned from work items will be
// handled in their own goroutine.
func (g Group) handleErrors(async bool, handler ErrorHandler) {
	for {
		select {

		// Clean up the goroutine when the group has died.
		case <-g.Death():
			return

		// Handle the error that were not handled by work items.
		case err := <-g.errChan:

			// Check to make sure the group isn't dead and this case was selected.
			if g.Dead() {
				return
			}

			// Handle the error async or not.
			if async {
				go handler(g, err)
			} else {
				handler(g, err)
			}
		}
	}
}

// mimic waits for all workers to be done working or for the group to die. Close the given channel, if any, when one
// condition occurs.
func (g Group) mimic(c chan struct{}) {

	// Close the channel, if any, after the function returns.
	defer func() {
		if c != nil {
			close(c)
		}
	}()

	// Check to see if the group is already dead.
	if g.Dead() {
		return
	}

	// Make a channel to wait for all workers to be done.
	done := make(chan struct{})

	// Launch a goroutine to wait for all workers to be done.
	go func() {
		g.wg.Wait()
		close(done)
	}()

	// Wait for a condition.
	select {
	case <-done:
	case <-g.death:
	}
}

// sendWorkItem adds to the work item channel's buffer or send the work directly to a worker if there is no buffer.
func (g Group) sendWorkItem(ctx context.Context, item *workItem) {

	// Make sure the context is not dead on arrival.
	if err := expired(item.ctx); err != nil {
		g.errChan <- ErrCantDo
		item.finished()
		return
	}

	// Send the work or fail to do so.
	select {
	case <-ctx.Done():
		g.errChan <- ErrCantDo
		item.finished()
		return
	case <-g.death:
		item.finished()
		return
	case g.do <- item:
	}
}
