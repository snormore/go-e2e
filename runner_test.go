package e2e

import (
	"testing"
)

func TestTestRunner(t *testing.T) {
	runner, err := NewTestRunner(
		WithTestDir("example"),
		WithDockerfile("Dockerfile"),
		WithBuildTags("e2e"),
		WithParallelism(1),
		WithVerbose(true),
	)
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
