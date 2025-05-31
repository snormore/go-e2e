package e2e

import (
	"testing"
)

func TestTestRunner(t *testing.T) {
	runner, err := NewTestRunner(TestRunnerConfig{
		TestDir:     "example",
		Dockerfile:  "Dockerfile",
		BuildTags:   "e2e",
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
