package e2e

import (
	"context"
	"fmt"
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
	containerBuildImage = "e2e-test-runner:dev"
	tmpDirPrefix        = "e2e-test-runner"
)

type TestRunner struct {
	testDir     string
	dockerfile  string
	testAssets  []string
	noFastFail  bool
	noParallel  bool
	parallelism int
	verbose     bool
	tmpDir      string
	assetsDir   string
	binDir      string
}

func NewTestRunner(testDir, dockerfile, testAssets string, verbose, noFastFail, noParallel bool, parallelism int) *TestRunner {
	return &TestRunner{
		testDir:     testDir,
		dockerfile:  dockerfile,
		testAssets:  strings.Split(testAssets, ","),
		noFastFail:  noFastFail,
		noParallel:  noParallel,
		parallelism: parallelism,
		verbose:     verbose,
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
	r.assetsDir = filepath.Join(r.tmpDir, "assets")
	if err := os.MkdirAll(r.assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %v", err)
	}
	if err := r.copyAssets(); err != nil {
		return fmt.Errorf("failed to copy assets: %v", err)
	}

	// Initialize the binary directory and build the test binary.
	r.binDir = filepath.Join(r.tmpDir, "bin")
	if err := os.MkdirAll(r.binDir, 0755); err != nil {
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
		fmt.Printf("--- INFO: Copying %s to %s\n", assetPath, filepath.Join(r.assetsDir, asset))
		if err := exec.Command("cp", "-r", assetPath, filepath.Join(r.assetsDir, asset)).Run(); err != nil {
			return fmt.Errorf("failed to copy %s: %v", asset, err)
		}
	}
	return nil
}

func (r *TestRunner) buildTestBinary() error {
	fmt.Printf("--- INFO: Building test binary in %s...\n", r.binDir)
	buildCmd := exec.Command("go", "test", r.testDir, "-c", "-o", filepath.Join(r.binDir, "run-test"))
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build test binary: %v", err)
	}
	return nil
}

func (r *TestRunner) buildDockerImage() error {
	if _, err := os.Stat(r.dockerfile); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found at %s", r.dockerfile)
	}

	dockerfilePath := filepath.Join(r.tmpDir, "Dockerfile")
	if err := exec.Command("cp", r.dockerfile, dockerfilePath).Run(); err != nil {
		return fmt.Errorf("failed to copy Dockerfile: %v", err)
	}

	fmt.Println("--- INFO: Building docker image (this may take a while)...")
	start := time.Now()
	buildDockerCmd := exec.Command("docker", "build",
		"--build-arg", "TEST_BIN=bin/run-test",
		"--build-arg", "TEST_ASSETS=assets",
		"-t", containerBuildImage,
		"-f", dockerfilePath,
		r.tmpDir)
	output, err := buildDockerCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build docker image\n%s", output)
	}
	fmt.Printf("--- OK: docker build (%.2fs)\n", time.Since(start).Seconds())
	return nil
}

func (r *TestRunner) getTestsToRun() ([]string, error) {
	testListCmd := exec.Command(filepath.Join(r.binDir, "run-test"), "-test.list", ".")
	testListOutput, err := testListCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get test list: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(testListOutput)), "\n"), nil
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
	fmt.Printf("--- INFO: Running %d tests %s...\n", len(testsToRun), map[bool]string{true: "sequentially", false: fmt.Sprintf("in parallel (max %d)", r.parallelism)}[r.noParallel])

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
		"--cap-add", "CAP_SYS_ADMIN",
		"--cap-add", "NET_ADMIN",
		containerBuildImage,
		"-test.run", fmt.Sprintf("^%s$", test))

	if r.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
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
			fmt.Printf("--- FAIL: %s (%.2fs)\n", test, testTimings[test].Seconds())
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
