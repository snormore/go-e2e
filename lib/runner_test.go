package e2e_test

import (
	"strings"
	"testing"

	e2e "github.com/snormore/go-e2e/lib"
)

func TestSuiteRunner_SimplePassing(t *testing.T) {
	runner, err := e2e.NewRunner(e2e.RunnerConfig{
		TestDir:     "../examples/simple-passing",
		Dockerfile:  "Dockerfile",
		Parallelism: 1,
		Verbosity:   2,
	})
	if err != nil {
		t.Fatalf("failed to create test runner: %v", err)
	}

	if err := runner.Setup(); err != nil {
		t.Fatalf("failed to setup test runner: %v", err)
	}
	defer runner.Cleanup()

	if err := runner.RunTests(); err != nil {
		t.Fatalf("failed to run tests: %v", err)
	}
}

func TestSuiteRunner_SimpleFailing(t *testing.T) {
	runner, err := e2e.NewRunner(e2e.RunnerConfig{
		TestDir:     "../examples/simple-failing",
		Dockerfile:  "Dockerfile",
		Parallelism: 1,
		Verbosity:   2,
	})
	if err != nil {
		t.Fatalf("failed to create test runner: %v", err)
	}

	if err := runner.Setup(); err != nil {
		t.Fatalf("failed to setup test runner: %v", err)
	}
	defer runner.Cleanup()

	if err := runner.RunTests(); err == nil || !strings.Contains(err.Error(), "tests failed") {
		t.Fatalf("expected test to fail with error containing 'tests failed' but got: %v", err)
	}
}
