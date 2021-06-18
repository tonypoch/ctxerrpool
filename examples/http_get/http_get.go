package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/MicahParks/ctxerrpool"
)

func main() {

	// Create a logger.
	l := log.New(os.Stdout, "", 0)

	// Create an HTTP client.
	httpClient := &http.Client{}

	// Define the URL string to GET.
	urlString := "http://golang.org"

	// Create the work function via a closure.
	var work ctxerrpool.Work
	work = func(ctx context.Context) (err error) {

		// Do the HTTP request, respect the given context.
		body, err := makeRequest(ctx, httpClient, urlString)
		if err != nil {
			return err
		}

		// Print the response body.
		l.Println(strings.TrimSpace(string(body)))

		return nil
	}

	// Create an error handler to log errors.
	var errorHandler ctxerrpool.ErrorHandler
	errorHandler = func(pool ctxerrpool.Pool, err error) {
		l.Printf("An error occurred: \"%v\".\n", err)
	}

	// Create a worker pool with 1 worker.
	pool := ctxerrpool.New(1, errorHandler)

	// Create a context for a some work.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)

	// Give the pool some work to do.
	pool.AddWorkItem(ctx, cancel, work)

	// Wait for the worker pool to be done working.
	pool.Wait()

	// Kill the worker pool when done using it.
	//
	// This isn't required here, because the main goroutine is about to return, but is typically a good idea.
	pool.Kill()
}

// makeRequest performs an HTTP GET request to the given URL using the given HTTP client. It will print the output to
// the logger and respect the given context.
func makeRequest(ctx context.Context, httpClient *http.Client, urlString string) (body []byte, err error) {

	// Create the HTTP request using the context.
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, urlString, bytes.NewBuffer([]byte{})); err != nil {
		return nil, err
	}

	// Do the HTTP request and get the response.
	var resp *http.Response
	if resp, err = httpClient.Do(req); err != nil {
		return nil, err
	}

	// Read the body of the response into a variable in the stack.
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, err
	}

	// Close the body of the response.
	if err = resp.Body.Close(); err != nil {
		return nil, err
	}

	return body, nil
}
