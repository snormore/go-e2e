package suite_test

import (
	"testing"

	"github.com/snormore/go-e2e/pkg/suite"
)

func TestSuiteRunner(t *testing.T) {
	runner, err := suite.NewRunner(suite.RunnerConfig{
		TestDir:     "../../example",
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
