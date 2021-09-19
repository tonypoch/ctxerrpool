package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"ctxerrpool"
)

const (

	// crawlDuration is how long the web scraper should scrape stuff.
	crawlDuration = time.Second * 5
)

var (

	// re matches some href tags.
	re = regexp.MustCompile(`<a\s+(?:[^>]*?\s+)?href="(.*?)"`)
)

// createContext creates a context and its cancellation function based on the amount of time scraping should happen.
func createContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), crawlDuration)
}

// handleHref takes in an href tag and adds it to the upcoming work for the crawler.
func handleHref(httpClient *http.Client, l *log.Logger, match []byte, pool ctxerrpool.Pool, startU *url.URL) {

	// Get the href's content as an absolute URL.
	aTag := string(match)
	split := strings.Split(aTag, `"`)
	nextURL := split[len(split)-2]
	nextU, err := url.Parse(nextURL)
	if err != nil {
		return
	}
	if !nextU.IsAbs() {
		if nextU, err = startU.Parse(nextU.String()); err != nil {
			return
		}
	}

	// Create a context for the next web crawling request.
	workerCtx, _ := createContext() // Be careful about shadowing variable names.
	// It's important to use the context.CancelFunc in production due to resource leaks.

	// Tell the worker pool to crawl to the next page.
	//
	// This is an example of how to create a work function via an anonymous function closure.
	go pool.AddWorkItem(workerCtx, func(workCtx context.Context, data interface{}) error {

		// Do the HTTP request and start crawling. Respect the given context.
		//
		// Make sure to use workCtx from anonymous function argument.
		if err := crawl(workCtx, httpClient, l, pool, data.(string)); err != nil {
			return err
		}

		return nil
	}, nextU.String())
}

// Don't make a web crawler like this, use github.com/gocolly/colly.
func main() {

	// Create a logger.
	l := log.New(os.Stdout, "", 0)

	// Create an HTTP client.
	httpClient := &http.Client{}

	// Define the URL string to GET.
	startURL := "http://golang.org"

	// Create an error handler to log errors.
	var errorHandler ctxerrpool.ErrorHandler
	errorHandler = func(pool ctxerrpool.Pool, err error) {
		l.Printf("An error occurred: \"%v\".\n", err)
	}

	// Create a worker pool with 4 workers.
	pool := ctxerrpool.New(4, errorHandler)

	// Create the work function via a closure.
	var work ctxerrpool.Work
	work = func(ctx context.Context, data interface{}) (err error) {

		// Do the HTTP request and start crawling. Respect the given context.
		if err := crawl(ctx, httpClient, l, pool, data.(string)); err != nil {
			return err
		}

		return nil
	}

	// Create a context for the first job.
	ctx, cancel := createContext()
	defer cancel()

	// Start the scraper.
	pool.AddWorkItem(ctx, work, startURL)

	// Wait for the pool to die or for the allowed amount of time to pass.
	select {
	case <-pool.Death():
	case <-time.After(crawlDuration):
		l.Println("This isn't meant to be a real crawler.")
	}
}

func crawl(ctx context.Context, httpClient *http.Client, l *log.Logger, pool ctxerrpool.Pool, urlString string) (err error) {

	// Make a url.Url from the given string.
	var startU *url.URL
	if startU, err = url.Parse(urlString); err != nil {
		return err
	}

	// Create the HTTP request using the context.
	var req *http.Request
	if req, err = http.NewRequestWithContext(ctx, http.MethodGet, startU.String(), bytes.NewBuffer([]byte{})); err != nil {
		return err
	}

	// Do the HTTP request and get the response.
	var resp *http.Response
	if resp, err = httpClient.Do(req); err != nil {
		return err
	}
	defer resp.Body.Close() // Ignore any error.

	// Read the body of the response into a variable in the stack.
	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return err
	}

	// Log the page as a success.
	l.Printf("Successfully retrieved URL: %s\n", urlString)

	// Find href tags.
	if matches := re.FindAll(body, -1); matches != nil {

		// For every match, get its link and crawl to it.
		for _, match := range matches {
			handleHref(httpClient, l, match, pool, startU)
		}
	}

	return nil
}
