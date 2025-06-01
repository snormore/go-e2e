package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/snormore/go-e2e"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	defaultParallelism = runtime.NumCPU()
)

func main() {
	var configFile string
	var verbosity int
	var noFastFail bool
	var noParallel bool
	var parallelism int

	config := e2e.TestRunnerConfig{}

	preprocessArgsForVerbosity()

	rootCmd := &cobra.Command{
		Use:   "go-e2e",
		Short: "Run containerized end-to-end tests",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			runner, err := e2e.NewTestRunner(
				e2e.TestRunnerConfig{
					TestDir:       ".",
					Dockerfile:    config.Dockerfile,
					TestAssets:    config.TestAssets,
					Verbosity:     config.Verbosity,
					NoFastFail:    config.NoFastFail,
					NoParallel:    config.NoParallel,
					Parallelism:   config.Parallelism,
					BuildTags:     config.BuildTags,
					DockerRunArgs: config.DockerRunArgs,
				},
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

	rootCmd.Flags().StringVarP(&configFile, "config", "f", "e2e.yaml", "Path to config file (YAML)")
	rootCmd.Flags().CountVarP(&verbosity, "verbose", "v", "Verbosity level. Can be specified multiple times to increase verbosity.")
	rootCmd.Flags().BoolVar(&noFastFail, "no-fast-fail", false, "Run all tests even if one fails")
	rootCmd.Flags().BoolVar(&noParallel, "no-parallel", false, "Run tests sequentially instead of in parallel")
	rootCmd.Flags().IntVarP(&parallelism, "parallelism", "p", defaultParallelism, "Number of tests to run in parallel")

	// Parse flags first to get config file path
	if err := rootCmd.ParseFlags(os.Args[1:]); err != nil {
		fmt.Printf("--- ERROR: %v\n", err)
		os.Exit(1)
	}

	// Set the flags-only config values.
	config.Verbosity = verbosity
	config.NoFastFail = noFastFail
	config.NoParallel = noParallel
	config.Parallelism = parallelism

	// Load config if specified
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Printf("--- INFO: Config file not found, using defaults: %v\n", err)
		} else {
			fmt.Printf("--- INFO: Using config file: %s\n", configFile)

			if err := yaml.Unmarshal(data, &config); err != nil {
				fmt.Printf("--- ERROR: Failed to parse config file: %v\n", err)
				os.Exit(1)
			}
		}
	}

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
