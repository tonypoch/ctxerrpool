package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (

	// The regex used to determine if the test was successful.
	noFail = regexp.MustCompile(`PASS
coverage: \d*\.\d*% of statements
ok  	github\.com/MicahParks/ctxerrpool	0\.\d*s`)  // Tests longer than 1 second are unacceptable.

	// The regex used to extract the coverage as a float.
	coverage = regexp.MustCompile(`coverage: \d*\.\d*%`)
)

func main() {

	// Create the log file.
	var logFile *os.File
	var err error
	if logFile, err = os.Create("cmd/profiles/log.txt"); err != nil {
		log.Fatalf("Could not create log file. Error: %s\n", err.Error())
	}
	defer func() {
		if err = logFile.Close(); err != nil {
			log.Printf("Could not close log file. Error: %s\n", err.Error())
		}
	}()

	// Create a logger.
	l := log.New(logFile, "", 0)

	// Make a channel to catch the signal from the OS.
	ctrlC := make(chan os.Signal, 1)

	// Tell the program to monitor for an interrupt or SIGTERM and report it on the given channel.
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)

	// Keep track of the total number of tests.
	total := 0

	// Keep track of the coverages seen.
	seen := make(map[float64]uint)

	// The file to save the coverage profile to.
	coverageOut := "cmd/profiles/coverage.out"

	// Find out how many tests to run.
	run := 0
	if len(os.Args) >= 2 {
		if run, err = strconv.Atoi(os.Args[1]); err != nil {
			run = 0
		}
	}
	if run != 0 {
		l.Printf("Running %d tests.\n", run)
	} else {
		l.Println("Running infinity tests.")
	}

	// Do the tests until told to stop.
	for i := 0; run == 0 || i < run; i++ {
		if runTest(coverageOut, ctrlC, l, seen, &total) {
			break
		}
	}

	// Print the end of tests info.
	l.Printf("Did %d tests:\n", total)
	for covers, count := range seen {
		l.Printf("  coverage %.2f%%: seen %d times", covers, count)
	}

	// Remove the generic coverage profile.
	if err = os.Remove(coverageOut); err != nil {
		l.Printf("Could not remove generic coverage file. Error: %s\n", err.Error())
	}
}

// getCoverage extracts the coverage as a float from the output.
func getCoverage(output []byte) (covers float64, err error) {

	// Use regex to get a more manageable portion of output.
	matches := coverage.FindStringSubmatch(string(output))

	// There should only be 1 match.
	if len(matches) != 1 {
		return 0, fmt.Errorf("incorrect number of matches: %d", len(matches))
	}

	// Get the float as a string.
	floatStr := strings.TrimSuffix(strings.TrimPrefix(matches[0], "coverage: "), "%")

	// Convert the float string to a float.
	return strconv.ParseFloat(floatStr, 64)
}

// saveCoverage renames the current coverage profile to one prefixed with it's amount of coverage.
func saveCoverage(covers float64, coverageOut string) (err error) {
	return os.Rename(coverageOut, fmt.Sprintf("%s/%.2f%s", filepath.Dir(coverageOut), covers, "coverage.out"))
}

// runTest runs a test of the code with coverage and the race detector.
func runTest(coverageOut string, ctrlC chan os.Signal, l *log.Logger, seen map[float64]uint, total *int) (done bool) {

	// Declare an error variable.
	var err error

	// Create the test command with coverage and race detection.
	cmd := exec.Command("go", "test", "-coverprofile", coverageOut, "-race")

	// Run the test command.
	var output []byte
	if output, err = cmd.CombinedOutput(); err != nil {
		l.Println(string(output))
		l.Printf("An error when running a test. Error: %s\n", err.Error())
		return true
	}

	// If the output does not match the regex, there was a race condition or test failure.
	if !noFail.Match(output) {
		l.Println(string(output))
		l.Println("Output did not match.")
		return true
	}

	// Extract the coverage float from the output.
	covers, err := getCoverage(output)
	if err != nil {
		l.Println(string(output))
		l.Println(err.Error())
		return true
	}

	// Save the coverage profile if it's new.
	_, ok := seen[covers]
	if !ok {
		if err := saveCoverage(covers, coverageOut); err != nil {
			l.Println("Could not save new coverage profile.")
			l.Println(err.Error())
			return true
		}
	}

	// Keep track of the coverage count.
	seen[covers]++

	// Increment the total number of tests.
	*total++

	// Print some info so the user knows it's not hanging.
	l.Printf("Test number %d: coverage %.2f%%: seen %d times", *total, covers, seen[covers])

	// Check for Ctrl + C.
	select {
	case <-ctrlC:
		l.Println("Got Ctrl + C.")
		return true
	default:
	}

	return false
}
