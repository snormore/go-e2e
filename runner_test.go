package e2e

import (
	"testing"
)

func TestTestRunner(t *testing.T) {
	runner := NewTestRunner("example", "Dockerfile", "", false, false, false, 1, "e2e")

	if err := runner.Setup(); err != nil {
		t.Fatalf("failed to setup test runner: %v", err)
	}
	defer runner.Cleanup()

	if err := runner.RunTests(); err != nil {
		t.Fatalf("failed to run tests: %v", err)
	}
}
