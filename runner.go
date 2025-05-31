package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// TODO: This should have a random suffix and get cleaned up after.
	containerBuildImage = "e2e-test-runner:dev"
	tmpDirPrefix        = "e2e-test-runner"
)

// TODO: Use slog for logging.

type TestRunner struct {
	testDir      string
	dockerfile   string
	testAssets   []string
	noFastFail   bool
	noParallel   bool
	parallelism  int
	verbose      bool
	tmpDir       string
	tmpAssetsDir string
	tmpBinDir    string
	buildTags    string
}

func NewTestRunner(testDir, dockerfile, testAssets string, verbose, noFastFail, noParallel bool, parallelism int, buildTags string) *TestRunner {
	return &TestRunner{
		testDir:     testDir,
		dockerfile:  dockerfile,
		testAssets:  strings.Split(testAssets, ","),
		noFastFail:  noFastFail,
		noParallel:  noParallel,
		parallelism: parallelism,
		verbose:     verbose,
		buildTags:   buildTags,
	}
}

func (r *TestRunner) Setup() error {
	var err error

	// Initialize the temporary directory.
	r.tmpDir, err = os.MkdirTemp("", tmpDirPrefix+"-*")
	if err != nil {
		return fmt.Errorf("failed to create tmp directory: %v", err)
	}

	// Initialize the assets directory and copy the assets.
	r.tmpAssetsDir = filepath.Join(r.tmpDir, "assets")
	if err := os.MkdirAll(r.tmpAssetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %v", err)
	}
	if err := r.copyAssets(); err != nil {
		return fmt.Errorf("failed to copy assets: %v", err)
	}

	// Initialize the binary directory and build the test binary.
	r.tmpBinDir = filepath.Join(r.tmpDir, "bin")
	if err := os.MkdirAll(r.tmpBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %v", err)
	}
	if err := r.buildTestBinary(); err != nil {
		return fmt.Errorf("failed to build test binary: %v", err)
	}

	// Build the docker image.
	if err := r.buildDockerImage(); err != nil {
		return fmt.Errorf("failed to build docker image: %v", err)
	}

	return nil
}

func (r *TestRunner) Cleanup() error {
	return os.RemoveAll(r.tmpDir)
}

func (r *TestRunner) copyAssets() error {
	for _, asset := range r.testAssets {
		asset = strings.TrimSpace(asset)
		if asset == "" {
			continue
		}
		assetPath := r.testDir + "/" + asset
		fmt.Printf("--- INFO: Copying %s to %s\n", assetPath, filepath.Join(r.tmpAssetsDir, asset))
		if err := exec.Command("cp", "-r", assetPath, filepath.Join(r.tmpAssetsDir, asset)).Run(); err != nil {
			return fmt.Errorf("failed to copy %s: %v", asset, err)
		}
	}
	return nil
}

func (r *TestRunner) buildTestBinary() error {
	fmt.Printf("--- INFO: Building test binary in %s...\n", r.tmpBinDir)
	args := []string{"test", "-c", "-o", filepath.Join(r.tmpBinDir, "run-test"), "."}
	if r.buildTags != "" {
		args = append(args, "-tags", r.buildTags)
	}
	buildCmd := exec.Command("go", args...)
	buildCmd.Dir = r.testDir
	buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	fmt.Printf("--- INFO: Running %s\n", strings.Join(buildCmd.Args, " "))
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build test binary: %v\n%s", err, string(output))
	}

	return nil
}

func (r *TestRunner) buildDockerImage() error {
	localDockerfilePath := filepath.Join(r.testDir, r.dockerfile)
	if _, err := os.Stat(localDockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found at %s", localDockerfilePath)
	}

	tmpDockerfilePath := filepath.Join(r.tmpDir, "Dockerfile")
	if err := exec.Command("cp", localDockerfilePath, tmpDockerfilePath).Run(); err != nil {
		return fmt.Errorf("failed to copy Dockerfile: %v", err)
	}

	fmt.Println("--- INFO: Building docker image (this may take a while)...")
	start := time.Now()
	buildDockerCmd := exec.Command("docker", "build",
		"--build-arg", "TEST_BIN=bin/run-test",
		"--build-arg", "TEST_ASSETS=assets",
		"-t", containerBuildImage,
		"-f", tmpDockerfilePath,
		r.tmpDir)
	buildDockerCmd.Dir = r.tmpDir
	output, err := buildDockerCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build docker image\n%s", output)
	}
	fmt.Printf("--- OK: docker build (%.2fs)\n", time.Since(start).Seconds())
	return nil
}

func (r *TestRunner) getTestsToRun() ([]string, error) {
	args := []string{"test", "-list", "."}
	if r.buildTags != "" {
		args = append(args, "-tags", r.buildTags)
	}
	testListCmd := exec.Command("go", args...)
	testListCmd.Dir = r.testDir
	testListOutput, err := testListCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get test list: %v\n%s", err, string(testListOutput))
	}
	testsToRun := strings.Split(strings.TrimSpace(string(testListOutput)), "\n")
	testsToRunLen := len(testsToRun)
	if testsToRunLen > 0 && strings.HasPrefix(testsToRun[testsToRunLen-1], "ok") {
		testsToRun = testsToRun[:testsToRunLen-1]
	}
	return testsToRun, nil
}

func (r *TestRunner) RunTests() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testsToRun, err := r.getTestsToRun()
	if err != nil {
		return fmt.Errorf("failed to get tests to run: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var failedTests []string
	var passedTests []string
	var incompleteTests []string
	var testTimings = make(map[string]time.Duration)

	suiteStart := time.Now()
	if len(testsToRun) == 1 {
		fmt.Printf("--- INFO: Running 1 test...\n")
	} else if len(testsToRun) > 1 {
		fmt.Printf("--- INFO: Running %d tests %s...\n", len(testsToRun), map[bool]string{true: "sequentially", false: fmt.Sprintf("in parallel (max %d)", r.parallelism)}[r.noParallel])
	} else {
		fmt.Printf("--- INFO: No tests to run.\n")
	}

	sem := make(chan struct{}, r.parallelism)

	for _, test := range testsToRun {
		if r.noParallel {
			r.runTest(test, &mu, &failedTests, &passedTests, &incompleteTests, testTimings, !r.noFastFail, ctx, cancel, testsToRun)
		} else {
			wg.Add(1)
			go func(test string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				r.runTest(test, &mu, &failedTests, &passedTests, &incompleteTests, testTimings, !r.noFastFail, ctx, cancel, testsToRun)
			}(test)
		}
	}

	if !r.noParallel {
		wg.Wait()
	}
	suiteDuration := time.Since(suiteStart)

	r.printSummary(suiteDuration, failedTests, passedTests, incompleteTests, testTimings)

	return nil
}

func (r *TestRunner) runTest(test string, mu *sync.Mutex, failedTests, passedTests, incompleteTests *[]string, testTimings map[string]time.Duration, failFast bool, ctx context.Context, cancel context.CancelFunc, testsToRun []string) {
	fmt.Printf("=== RUN: %s\n", test)
	start := time.Now()

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"--name", sanitizeContainerName(test),
		"--entrypoint", "/bin/run-test",
		containerBuildImage,
		"-test.run", fmt.Sprintf("^%s$", test))
	cmd.Dir = r.tmpDir

	var output bytes.Buffer
	if r.verbose {
		cmd.Stdout = io.MultiWriter(os.Stdout, &output)
		cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	if err := cmd.Run(); err != nil {
		mu.Lock()
		if len(*failedTests) == 0 {
			if failFast {
				cancel()
				for _, t := range testsToRun {
					t = strings.TrimSpace(t)
					if t == "" || t == test {
						continue
					}
					ran := false
					for _, pt := range *passedTests {
						if pt == t {
							ran = true
							break
						}
					}
					for _, ft := range *failedTests {
						if ft == t {
							ran = true
							break
						}
					}
					if !ran {
						*incompleteTests = append(*incompleteTests, t)
					}
				}
			}
		}
		*failedTests = append(*failedTests, test)
		testTimings[test] = time.Since(start)
		mu.Unlock()
		if test == (*failedTests)[0] {
			if r.verbose {
				fmt.Printf("--- FAIL: %s (%.2fs)\n", test, testTimings[test].Seconds())
			} else {
				fmt.Printf("--- FAIL: %s (%.2fs)\n%s", test, testTimings[test].Seconds(), output.String())
			}
		}
	} else {
		mu.Lock()
		*passedTests = append(*passedTests, test)
		testTimings[test] = time.Since(start)
		mu.Unlock()
		fmt.Printf("--- PASS: %s (%.2fs)\n", test, testTimings[test].Seconds())
	}
}

func (r *TestRunner) printSummary(suiteDuration time.Duration, failedTests, passedTests, incompleteTests []string, testTimings map[string]time.Duration) {
	fmt.Println()
	if len(failedTests) == 0 {
		fmt.Printf("=== SUMMARY: PASS (%.2fs)\n", suiteDuration.Seconds())
		for _, test := range passedTests {
			fmt.Printf("PASS: %s (%.2fs)\n", test, testTimings[test].Seconds())
		}
	} else {
		fmt.Printf("=== SUMMARY: FAIL (%.2fs)\n", suiteDuration.Seconds())
		for _, test := range passedTests {
			fmt.Printf("PASS: %s (%.2fs)\n", test, testTimings[test].Seconds())
		}
		if !r.noFastFail {
			for _, test := range failedTests {
				fmt.Printf("FAIL: %s (%.2fs)\n", test, testTimings[test].Seconds())
			}
		} else {
			fmt.Printf("FAIL: %s (%.2fs)\n", failedTests[0], testTimings[failedTests[0]].Seconds())
			for _, test := range incompleteTests {
				fmt.Printf("STOP: %s\n", test)
			}
		}
	}
}

// sanitizeContainerName converts a test name to a valid Docker container name
func sanitizeContainerName(testName string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	name := reg.ReplaceAllString(testName, "-")
	if !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
		name = "e2e-" + name
	}
	if len(name) > 20 {
		name = name[:20]
	}
	suffix := fmt.Sprintf("%04x", rand.Intn(65536))
	return fmt.Sprintf("e2e-%s-%s", name, suffix)
}
