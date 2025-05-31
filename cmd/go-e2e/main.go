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
		verbosity     int
		noFastFail    bool
		noParallel    bool
		parallelism   int
		buildTags     string
		dockerRunArgs []string
	)

	preprocessArgsForVerbosity()

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
				e2e.WithTestAssets(testAssets),
				e2e.WithVerbosity(verbosity),
				e2e.WithNoFastFail(noFastFail),
				e2e.WithNoParallel(noParallel),
				e2e.WithParallelism(parallelism),
				e2e.WithBuildTags(buildTags),
				e2e.WithDockerRunArgs(dockerRunArgs),
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

	// TODO: Support an e2e.yaml config file for all of this.

	rootCmd.Flags().StringVar(&dockerfile, "dockerfile", "Dockerfile", "Path to the Dockerfile to use to run the tests")
	rootCmd.Flags().StringArrayVar(&testAssets, "test-asset", []string{}, "Test assets to copy from the test directory. You can use this multiple times to add multiple assets.")
	rootCmd.Flags().CountVarP(&verbosity, "verbose", "v", "Verbosity level. Can be specified multiple times to increase verbosity.")
	rootCmd.Flags().BoolVar(&noFastFail, "no-fast-fail", false, "Run all tests even if one fails")
	rootCmd.Flags().BoolVar(&noParallel, "no-parallel", false, "Run tests sequentially instead of in parallel")
	rootCmd.Flags().IntVarP(&parallelism, "parallelism", "p", runtime.NumCPU(), "Number of tests to run in parallel")
	rootCmd.Flags().StringVar(&buildTags, "build-tags", "e2e", "Build tags to use when building the test binary")
	rootCmd.Flags().StringArrayVar(&dockerRunArgs, "docker-run-arg", []string{}, "Arguments to pass to the docker run command when running the tests. You can use this multiple times to add multiple arguments.")

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("--- ERROR: %v\n", err)
		os.Exit(1)
	}
}

func preprocessArgsForVerbosity() {
	newArgs := []string{os.Args[0]}
	for _, arg := range os.Args[1:] {
		switch {
		case strings.HasPrefix(arg, "-vvv") && len(arg) == 4:
			newArgs = append(newArgs, "--verbose=3")
		case strings.HasPrefix(arg, "-vv") && len(arg) == 3:
			newArgs = append(newArgs, "--verbose=2")
		case arg == "-v":
			newArgs = append(newArgs, "--verbose=1")
		default:
			newArgs = append(newArgs, arg)
		}
	}
	os.Args = newArgs
}
