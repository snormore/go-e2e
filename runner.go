package e2e

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
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

type TestRunner struct {
	testDir       string
	dockerfile    string
	testAssets    []string
	noFastFail    bool
	noParallel    bool
	parallelism   int
	verbosity     int
	tmpDir        string
	tmpAssetsDir  string
	tmpBinDir     string
	buildTags     string
	dockerRunArgs []string

	mu              sync.Mutex
	failedTests     []string
	passedTests     []string
	incompleteTests []string
	testTimings     map[string]time.Duration
	testsToRun      []string
}

func NewTestRunner(options ...Option) (*TestRunner, error) {
	runner := &TestRunner{}
	for _, option := range options {
		option(runner)
	}

	// Validate required options.
	if runner.testDir == "" {
		return nil, fmt.Errorf("testDir is required")
	}
	if runner.dockerfile == "" {
		return nil, fmt.Errorf("dockerfile is required")
	}

	return runner, nil
}

type Option func(*TestRunner)

func WithTestDir(testDir string) Option {
	return func(r *TestRunner) {
		r.testDir = testDir
	}
}

func WithDockerfile(dockerfile string) Option {
	return func(r *TestRunner) {
		r.dockerfile = dockerfile
	}
}

func WithTestAssets(testAssets []string) Option {
	return func(r *TestRunner) {
		r.testAssets = testAssets
	}
}

func WithNoFastFail(noFastFail bool) Option {
	return func(r *TestRunner) {
		r.noFastFail = noFastFail
	}
}

func WithNoParallel(noParallel bool) Option {
	return func(r *TestRunner) {
		r.noParallel = noParallel
	}
}

func WithParallelism(parallelism int) Option {
	return func(r *TestRunner) {
		r.parallelism = parallelism
	}
}

func WithVerbosity(verbosity int) Option {
	return func(r *TestRunner) {
		r.verbosity = verbosity
	}
}

func WithBuildTags(buildTags string) Option {
	return func(r *TestRunner) {
		r.buildTags = buildTags
	}
}

func WithDockerRunArgs(dockerRunArgs []string) Option {
	return func(r *TestRunner) {
		r.dockerRunArgs = dockerRunArgs
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

	// Get tests to run.
	r.testsToRun, err = r.getTestsToRun()
	if err != nil {
		return fmt.Errorf("failed to get tests to run: %v", err)
	}

	if r.verbosity > 0 {
		fmt.Printf("--- INFO: Running with verbosity %d\n", r.verbosity)
	}

	return nil
}

func (r *TestRunner) Cleanup() {
	_ = os.RemoveAll(r.tmpDir)
}

func (r *TestRunner) copyAssets() error {
	for _, asset := range r.testAssets {
		asset = strings.TrimSpace(asset)
		if asset == "" {
			continue
		}
		assetPath := r.testDir + "/" + asset
		if r.verbosity > 1 {
			fmt.Printf("--- DEBUG: Copying %s to %s\n", assetPath, filepath.Join(r.tmpAssetsDir, asset))
		}
		if err := exec.Command("cp", "-r", assetPath, filepath.Join(r.tmpAssetsDir, asset)).Run(); err != nil {
			return fmt.Errorf("failed to copy %s: %v", asset, err)
		}
	}
	return nil
}

func (r *TestRunner) buildTestBinary() error {
	if r.verbosity > 1 {
		fmt.Printf("--- DEBUG: Building test binary in %s\n", r.tmpBinDir)
	}
	args := []string{"test", "-c", "-o", filepath.Join(r.tmpBinDir, "run-test"), "."}
	if r.buildTags != "" {
		// TODO: Make sure this is working.
		args = append(args, "-tags", r.buildTags)
	}
	buildCmd := exec.Command("go", args...)
	buildCmd.Dir = r.testDir
	buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if r.verbosity > 1 {
		fmt.Printf("--- DEBUG: Running: %s\n", strings.Join(buildCmd.Args, " "))
	}
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
	var tests []string
	fset := token.NewFileSet()
	err := filepath.Walk(r.testDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, "_test.go") {
			// Parse the file for test functions and build constraints.
			f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %v", path, err)
			}

			// Check build tags.
			if r.buildTags != "" {
				buildTags := strings.Split(r.buildTags, ",")
				var buildConstraint constraint.Expr

				// Find build constraint in comments before package declaration.
				for _, cg := range f.Comments {
					for _, c := range cg.List {
						text := strings.TrimSpace(c.Text)
						if constraint.IsGoBuild(text) {
							buildConstraint, err = constraint.Parse(text)
							if err != nil {
								return fmt.Errorf("failed to parse build constraint %q: %v", text, err)
							}
							break
						}
					}

					// Stop early if the comment group ends before package declaration.
					if cg.End() >= f.Package {
						break
					}
				}

				if buildConstraint != nil {
					// Create a tag set for evaluation
					tagSet := make(map[string]bool)
					for _, tag := range buildTags {
						tagSet[tag] = true
					}

					if !buildConstraint.Eval(func(tag string) bool {
						return tagSet[tag]
					}) {
						return nil
					}
				}
			}

			for _, decl := range f.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if strings.HasPrefix(funcDecl.Name.Name, "Test") {
					tests = append(tests, funcDecl.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find tests: %v", err)
	}
	return tests, nil
}

func (r *TestRunner) RunTests() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	r.testTimings = make(map[string]time.Duration)

	suiteStart := time.Now()
	switch len(r.testsToRun) {
	case 1:
		fmt.Printf("--- INFO: Running 1 test...\n")
	case 0:
		fmt.Printf("--- INFO: No tests to run.\n")
	default:
		fmt.Printf("--- INFO: Running %d tests %s...\n", len(r.testsToRun), map[bool]string{true: "sequentially", false: fmt.Sprintf("in parallel (max %d)", r.parallelism)}[r.noParallel])
	}

	sem := make(chan struct{}, r.parallelism)

	for _, test := range r.testsToRun {
		if r.noParallel {
			r.runTest(ctx, test, cancel)
		} else {
			wg.Add(1)
			go func(test string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				r.runTest(ctx, test, cancel)
			}(test)
		}
	}

	if !r.noParallel {
		wg.Wait()
	}
	suiteDuration := time.Since(suiteStart)

	r.printSummary(suiteDuration)

	return nil
}

func (r *TestRunner) runTest(ctx context.Context, test string, cancel context.CancelFunc) {
	fmt.Printf("=== RUN: %s\n", test)
	start := time.Now()

	args := []string{"run", "--rm",
		"--name", sanitizeContainerName(test)}
	if len(r.dockerRunArgs) > 0 {
		for _, arg := range r.dockerRunArgs {
			args = append(args, strings.Fields(arg)...)
		}
	}
	args = append(args, containerBuildImage, "-test.run", fmt.Sprintf("^%s$", test))
	if r.verbosity > 0 {
		args = append(args, "-test.v")
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	if r.verbosity > 1 {
		fmt.Printf("--- DEBUG: Running: %s\n", strings.Join(cmd.Args, " "))
	}
	cmd.Dir = r.tmpDir

	var output bytes.Buffer
	if r.verbosity > 0 {
		cmd.Stdout = io.MultiWriter(os.Stdout, &output)
		cmd.Stderr = io.MultiWriter(os.Stderr, &output)
	} else {
		cmd.Stdout = &output
		cmd.Stderr = &output
	}

	if err := cmd.Run(); err != nil {
		r.mu.Lock()
		if len(r.failedTests) == 0 {
			if !r.noFastFail {
				cancel()
				for _, t := range r.testsToRun {
					t = strings.TrimSpace(t)
					if t == "" || t == test {
						continue
					}
					ran := false
					for _, pt := range r.passedTests {
						if pt == t {
							ran = true
							break
						}
					}
					for _, ft := range r.failedTests {
						if ft == t {
							ran = true
							break
						}
					}
					if !ran {
						r.incompleteTests = append(r.incompleteTests, t)
					}
				}
			}
		}
		r.failedTests = append(r.failedTests, test)
		r.testTimings[test] = time.Since(start)
		r.mu.Unlock()
		if test == r.failedTests[0] {
			if r.verbosity > 0 {
				fmt.Printf("--- FAIL: %s (%.2fs)\n", test, r.testTimings[test].Seconds())
			} else {
				fmt.Printf("--- FAIL: %s (%.2fs)\n%s", test, r.testTimings[test].Seconds(), output.String())
			}
		}
	} else {
		r.mu.Lock()
		r.passedTests = append(r.passedTests, test)
		r.testTimings[test] = time.Since(start)
		r.mu.Unlock()
		fmt.Printf("--- PASS: %s (%.2fs)\n", test, r.testTimings[test].Seconds())
	}
}

func (r *TestRunner) printSummary(suiteDuration time.Duration) {
	fmt.Println()
	if len(r.failedTests) == 0 {
		fmt.Printf("=== SUMMARY: PASS (%.2fs)\n", suiteDuration.Seconds())
		for _, test := range r.passedTests {
			fmt.Printf("PASS: %s (%.2fs)\n", test, r.testTimings[test].Seconds())
		}
	} else {
		fmt.Printf("=== SUMMARY: FAIL (%.2fs)\n", suiteDuration.Seconds())
		for _, test := range r.passedTests {
			fmt.Printf("PASS: %s (%.2fs)\n", test, r.testTimings[test].Seconds())
		}
		if !r.noFastFail {
			for _, test := range r.failedTests {
				fmt.Printf("FAIL: %s (%.2fs)\n", test, r.testTimings[test].Seconds())
			}
		} else {
			fmt.Printf("FAIL: %s (%.2fs)\n", r.failedTests[0], r.testTimings[r.failedTests[0]].Seconds())
			for _, test := range r.incompleteTests {
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
