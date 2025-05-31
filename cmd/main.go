package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/snormore/go-e2e"
)

var (
	// TODO: Consider using clap here and include a help command.
	testDir     = flag.String("test-dir", ".", "Path to the test directory")
	dockerfile  = flag.String("dockerfile", "Dockerfile", "Path to the Dockerfile to use to run the tests")
	testAssets  = flag.String("test-assets", "fixtures", "Comma-separated list of test assets to copy from the test directory")
	verbose     = flag.Bool("v", false, "Show verbose test output")
	noFastFail  = flag.Bool("no-fast-fail", false, "Run all tests even if one fails")
	noParallel  = flag.Bool("no-parallel", false, "Run tests sequentially instead of in parallel")
	parallelism = flag.Int("p", runtime.NumCPU(), "Number of tests to run in parallel")
)

func main() {
	flag.Parse()

	// Initialize the test runner.
	runner := e2e.NewTestRunner(*testDir, *dockerfile, *testAssets, *verbose, *noFastFail, *noParallel, *parallelism)
	if err := runner.Setup(); err != nil {
		exitWithError(err)
	}
	defer runner.Cleanup()

	// Find and run the tests.
	if err := runner.RunTests(); err != nil {
		exitWithError(err)
	}
}

func exitWithError(err error) {
	fmt.Printf("--- ERROR: %v\n", err)
	os.Exit(1)
}
