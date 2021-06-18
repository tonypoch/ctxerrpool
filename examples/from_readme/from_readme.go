package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MicahParks/ctxerrpool"
)

func main() {

	// Create an error handler that logs all errors.
	var errorHandler ctxerrpool.ErrorHandler
	errorHandler = func(pool ctxerrpool.Pool, err error) {
		log.Printf("An error occurred. Error: \"%s\".\n", err.Error())
	}

	// Create a worker pool with 4 workers.
	pool := ctxerrpool.New(4, errorHandler)

	// Create some variables to inherit through a closure.
	httpClient := &http.Client{}
	u := "https://golang.org"
	logger := log.New(os.Stdout, "status codes: ", 0)

	// Create the worker function.
	var work ctxerrpool.Work
	work = func(ctx context.Context) (err error) {

		// Create the HTTP request.
		var req *http.Request
		if req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, bytes.NewReader(nil)); err != nil {
			return err
		}

		// Do the HTTP request.
		var resp *http.Response
		if resp, err = httpClient.Do(req); err != nil {
			return err
		}

		// Log the status code.
		logger.Println(resp.StatusCode)

		return nil
	}

	// Do the work 16 times.
	for i := 0; i < 16; i++ {

		// Create a context for the work.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Send the work to the pool.
		pool.AddWorkItem(ctx, work)
	}

	// Wait for the pool to finish.
	pool.Wait()
}
