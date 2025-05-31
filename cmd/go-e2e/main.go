package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/snormore/go-e2e"
	"github.com/spf13/cobra"
)

func main() {
	var (
		dockerfile    string
		testAssets    []string
		verbose       bool
		noFastFail    bool
		noParallel    bool
		parallelism   int
		buildTags     string
		dockerRunArgs []string
	)

	rootCmd := &cobra.Command{
		Use:   "go-e2e [test-dir]",
		Short: "Run containerized end-to-end tests",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			testDir := "."
			if len(args) > 0 {
				testDir = args[0]
			}

			runner, err := e2e.NewTestRunner(
				e2e.WithTestDir(testDir),
				e2e.WithDockerfile(dockerfile),
				e2e.WithTestAssets(strings.Join(testAssets, ",")),
				e2e.WithVerbose(verbose),
				e2e.WithNoFastFail(noFastFail),
				e2e.WithNoParallel(noParallel),
				e2e.WithParallelism(parallelism),
				e2e.WithBuildTags(buildTags),
				e2e.WithDockerRunArgs(strings.Join(dockerRunArgs, " ")),
			)
			if err != nil {
				return err
			}
			if err := runner.Setup(); err != nil {
				return err
			}
			defer runner.Cleanup()

			return runner.RunTests()
		},
	}

	rootCmd.Flags().StringVarP(&dockerfile, "dockerfile", "f", "Dockerfile", "Path to the Dockerfile to use to run the tests")
	rootCmd.Flags().StringSliceVarP(&testAssets, "test-assets", "a", nil, "Test assets to copy from the test directory")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show verbose test output")
	rootCmd.Flags().BoolVar(&noFastFail, "no-fast-fail", false, "Run all tests even if one fails")
	rootCmd.Flags().BoolVar(&noParallel, "no-parallel", false, "Run tests sequentially instead of in parallel")
	rootCmd.Flags().IntVarP(&parallelism, "parallelism", "p", runtime.NumCPU(), "Number of tests to run in parallel")
	rootCmd.Flags().StringVar(&buildTags, "build-tags", "e2e", "Build tags to use when building the test binary")
	rootCmd.Flags().StringSliceVar(&dockerRunArgs, "docker-run-args", nil, "Arguments to pass to the docker run command when running the tests")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("--- ERROR: %v\n", err)
		os.Exit(1)
	}
}
