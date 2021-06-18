package ctxerrpool_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/MicahParks/ctxerrpool"
)

// TestDeathBeforeWork confirms that a worker pool can be killed before doing any work safely.
func TestDeathBeforeWork(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Kill the pool.
	pool.Kill()

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestDeathDeadOnArrival confirms that a worker pool can be killed and then given work, but no work will be performed.
func TestDeathDeadOnArrival(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Kill the pool.
	pool.Kill()

	// Create a context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Do some work with the pool.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		t.Fail() // This line should never run.
		return nil
	})

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestDeathDuring confirms that work will stop being performed if the pool dies and the given work respects it's own
// context.
func TestDeathDuring(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Create a context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a wait pool that indicates the work has started.
	workWg := &sync.WaitGroup{}
	workWg.Add(1)

	// Do some work with the pool.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		workWg.Done()

		// Respect given context.
		select {

		// Fail after the work has started, but before the context deadline.
		case <-time.After(time.Millisecond * 500):
			t.FailNow()

		// Catch the context's cancellation.
		case <-workCtx.Done():

			// This should be because the context was canceled.
			if err := workCtx.Err(); !errors.Is(err, context.Canceled) {
				t.Errorf("An error occurred. Error: %v", err)
				t.Fail()
			}
		}

		return nil
	})

	// Let the program set up the pool and start working.
	workWg.Wait()

	// Kill the pool.
	pool.Kill()

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestDone confirms that the Done method works as expected.
func TestDone(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Create a context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a boolean that indicates the work was done and a mutex for it.
	mux := &sync.Mutex{}
	done := false

	// Do some work with the pool.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		mux.Lock()
		defer mux.Unlock()
		done = true
		return nil
	})

	<-pool.Done()

	// Check to make sure that the pool.Done() channel blocked until the work was completed.
	mux.Lock()
	defer mux.Unlock()
	if !done {
		t.Error("The work was not completed.")
		t.FailNow()
	}

	wg.Wait()
}

// TestErrCantDo confirms the case where the given work's context expired before it was given to a worker is reported
// over the error channel with the reason being because other processes were using the pool. This is mimicked by having
// no workers in the pool.
func TestErrCantDo(t *testing.T) {

	// Create a wait pool that waits for the error to be handled.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create a worker pool with 0 workers.
	pool := ctxerrpool.New(0, func(pool ctxerrpool.Pool, err error) {
		defer wg.Done()

		// This test case should have the ctxerrpool.ErrCantDo error.
		if !errors.Is(err, ctxerrpool.ErrCantDo) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Create a context for the job.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	// Give the worker pool some work that will never get run.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		return nil
	})

	// Wait for the worker pool and error.
	wg.Wait()
	pool.Wait()
}

// TestErrCantDo confirms the case where the given work's context expired before it was given to a worker is reported
// over the error channel with the reason being because the context was expired upon arrival.
func TestErrCantDoDeadOnArrival(t *testing.T) {

	// Create a wait pool that waits for the error to be handled.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		defer wg.Done()

		// This test case should have the ctxerrpool.ErrCantDo error.
		if !errors.Is(err, ctxerrpool.ErrCantDo) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Create a context for the job that will be expired on arrival.
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	// Get a worker to do some work so the main loop is entered.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		return nil
	})

	// Wait for the worker pool and error.
	wg.Wait()
	pool.Wait()
}

// TestErrorTimeout confirms that work which respects it's own context will be canceled from within the work itself and
// its error will be reported to the pool.
func TestErrorTimeout(t *testing.T) {

	// Create a wait pool that waits for the error to be handled.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		defer wg.Done()

		// This test case should have the context.DeadlineExceeded error.
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Create a context for the job that will time out.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	// Get a worker to sleep for a second, but async wait for its context to expire.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		var err error
		select {
		case <-time.After(time.Second):
			t.FailNow()
		case <-workCtx.Done(): // Good case.
			err = workCtx.Err()
		}
		return err
	})

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestErrorTimeoutBadWork confirms the case when a context expires but the work does not respect its own context. The
// pool and it's worker should be usable, but the goroutine with the work will leak.  Don't do that. Respect the
// context.
func TestErrorTimeoutBadWork(t *testing.T) {

	// Create a wait pool that waits for the error to be handled.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		defer wg.Done()

		// This test case should have the context.DeadlineExceeded error.
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Create a context for the job that will time out.
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	// Get a worker to sleep for a second.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {

		// Do not respect workCtx.
		select {
		case <-time.After(time.Second * 10): // Fail after the tests should be done. The goroutine for this work will leak.
			t.FailNow()
		}
		return nil
	})

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestKill confirms that the Kill method behaves as expected.
func TestKill(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case may have the context.Canceled error.
		if !errors.Is(err, context.Canceled) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Get the current time.
	start := time.Now()

	// Create a context for the job that will time out.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Get a worker to sleep for a second, but async wait for its context to expire.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		var err error
		select {
		case <-time.After(time.Second):
			t.FailNow()
		case <-workCtx.Done():
			err = workCtx.Err()
		}
		return err
	})

	// Kill the pool right away.
	pool.Kill()

	// Wait for the pool to be killed.
	<-pool.Death()

	// The pool should not have done all the work.
	if time.Now().Sub(start) > time.Second {
		t.Errorf("The pool was killed, but did not interrupt the work properly.")
		t.FailNow()
	}

	// Wait for any errors.
	wg.Wait()
}

// TestMultiWorker confirms multi worker pools will work as expected.
func TestMultiWorker(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 2 workers.
	pool := ctxerrpool.New(2, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Two contexts. One for each job.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()

	// Grab the current time.
	start := time.Now()

	// Get both workers to sleep for 50 millisecond each. If it takes 100 or more milliseconds total, only one worker
	// was used.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		time.Sleep(time.Millisecond * 50)
		return nil
	})
	pool.AddWorkItem(ctx2, func(workCtx context.Context) error {
		time.Sleep(time.Millisecond * 50)
		return nil
	})

	// The worker pool is done working.
	pool.Wait()

	// Make sure that the total amount of time was not longer than 100 milliseconds.
	if duration := time.Now().Sub(start); duration >= time.Millisecond*100 {
		t.Errorf("Either the computer is very slow or only one worker from the pool was used.")
		t.FailNow()
	}

	// Wait for any errors.
	wg.Wait()
}

// TestNew confirms that a worker pool can be created with the New function.
func TestNew(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Create a context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Do some work with the pool.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		return nil
	})

	// Wait for the worker pool and error.
	pool.Wait()
	wg.Wait()
}

// TestWait confirms the Wait method behaves as expected.
func TestWait(t *testing.T) {

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		wg.Add(1)
		defer wg.Done()

		// This test case should have no error.
		t.Errorf("An error occurred. Error: %v", err)
		t.FailNow()
	})

	// Create a context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a boolean that indicates the work was done and a mutex for it.
	mux := &sync.Mutex{}
	done := false

	// Do some work with the pool.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		mux.Lock()
		defer mux.Unlock()
		done = true
		return nil
	})

	// Wait for all the work to be done.
	pool.Wait()

	// Check to make sure that the pool.Wait() call let all the work complete.
	mux.Lock()
	defer mux.Unlock()
	if !done {
		t.Error("Pool did not complete the work.")
		t.FailNow()
	}

	// Wait for any errors.
	wg.Wait()
}

// TestWorkerError confirms that if work returns an error that isn't associated with the ctxerrpool, it will be reported
// properly over the Pool's error channel.
func TestWorkerError(t *testing.T) {

	// Create an error that should be sent back async.
	customErr := io.EOF

	// Wait for errors if they are in the process of being handled.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, func(pool ctxerrpool.Pool, err error) {
		defer wg.Done()

		// This test case should have no error.
		if !errors.Is(err, customErr) {
			t.Errorf("An error occurred. Error: %v", err)
			t.FailNow()
		}
	})

	// Create a context for the job.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Get a worker to give a custom error async.
	pool.AddWorkItem(ctx, func(workCtx context.Context) error {
		return customErr
	})

	// Wait for the expected error.
	wg.Wait()
}
