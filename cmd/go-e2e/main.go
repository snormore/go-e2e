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

const (
	defaultDockerfile = "Dockerfile"
	defaultBuildTags  = "e2e"
)

var (
	defaultParallelism = runtime.NumCPU()
)

func main() {
	var configFile string
	config := e2e.TestRunnerConfig{}

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
				e2e.TestRunnerConfig{
					TestDir:       testDir,
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

	rootCmd.Flags().StringVarP(&configFile, "config", "f", "", "Path to config file (YAML)")
	rootCmd.Flags().StringVar(&config.Dockerfile, "dockerfile", defaultDockerfile, "Path to the Dockerfile to use to run the tests")
	rootCmd.Flags().StringArrayVar(&config.TestAssets, "test-asset", nil, "Test assets to copy from the test directory. You can use this multiple times to add multiple assets.")
	rootCmd.Flags().CountVarP(&config.Verbosity, "verbose", "v", "Verbosity level. Can be specified multiple times to increase verbosity.")
	rootCmd.Flags().BoolVar(&config.NoFastFail, "no-fast-fail", false, "Run all tests even if one fails")
	rootCmd.Flags().BoolVar(&config.NoParallel, "no-parallel", false, "Run tests sequentially instead of in parallel")
	rootCmd.Flags().IntVarP(&config.Parallelism, "parallelism", "p", defaultParallelism, "Number of tests to run in parallel")
	rootCmd.Flags().StringVar(&config.BuildTags, "build-tags", defaultBuildTags, "Build tags to use when building the test binary")
	rootCmd.Flags().StringArrayVar(&config.DockerRunArgs, "docker-run-arg", nil, "Arguments to pass to the docker run command when running the tests. You can use this multiple times to add multiple arguments.")

	// Parse flags first to get config file path
	if err := rootCmd.ParseFlags(os.Args[1:]); err != nil {
		fmt.Printf("--- ERROR: %v\n", err)
		os.Exit(1)
	}

	// Load config if specified
	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Printf("--- ERROR: Failed to read config file: %v\n", err)
			os.Exit(1)
		}

		var fileConfig e2e.TestRunnerConfig
		if err := yaml.Unmarshal(data, &fileConfig); err != nil {
			fmt.Printf("--- ERROR: Failed to parse config file: %v\n", err)
			os.Exit(1)
		}

		// Apply config values only if not set via flags
		if config.Dockerfile == defaultDockerfile && fileConfig.Dockerfile != "" {
			config.Dockerfile = fileConfig.Dockerfile
		}
		if len(config.TestAssets) == 0 && len(fileConfig.TestAssets) > 0 {
			config.TestAssets = fileConfig.TestAssets
		}
		if config.BuildTags == defaultBuildTags && fileConfig.BuildTags != "" {
			config.BuildTags = fileConfig.BuildTags
		}
		if len(config.DockerRunArgs) == 0 && len(fileConfig.DockerRunArgs) > 0 {
			config.DockerRunArgs = fileConfig.DockerRunArgs
		}
		if config.Verbosity == 0 && fileConfig.Verbosity > 0 {
			config.Verbosity = fileConfig.Verbosity
		}
		if !config.NoFastFail && fileConfig.NoFastFail {
			config.NoFastFail = fileConfig.NoFastFail
		}
		if !config.NoParallel && fileConfig.NoParallel {
			config.NoParallel = fileConfig.NoParallel
		}
		if config.Parallelism == defaultParallelism && fileConfig.Parallelism > 0 {
			config.Parallelism = fileConfig.Parallelism
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
