package main

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/MicahParks/ctxerrgroup"
)

func main() {

	// Create an error handler that logs all errors.
	var errorHandler ctxerrgroup.ErrorHandler
	errorHandler = func(group ctxerrgroup.Group, err error) {
		log.Printf("An error occurred. Error: \"%s\".\n", err.Error())
	}

	// Create a worker group with 8 workers and a buffer that can queue work functions. Do not handle errors in a
	// separate goroutine.
	group := ctxerrgroup.New(4, 8, false, errorHandler)

	// Create some variables to inherit through a closure.
	httpClient := &http.Client{}
	u := "https://golang.org"
	logger := log.New(os.Stdout, "status codes: ", 0)

	// Create the worker function.
	var work ctxerrgroup.Work
	work = func(ctx context.Context) (err error) {

		// Create the HTTP request.
		var req *http.Request
		if req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, bytes.NewReader([]byte{})); err != nil {
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

		// Send the work to the group.
		group.AddWorkItem(ctx, cancel, work)
	}

	// Wait for the group to finish.
	group.Wait()
}
