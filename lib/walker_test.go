package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	e2e "github.com/snormore/go-e2e/lib"
)

func TestFileWalker(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "e2e-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directories with config files
	dirs := []string{
		"dir1",
		"dir1/subdir1",
		"dir2",
		"dir2/subdir2",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// Create config files in some directories
	configFiles := []string{
		"dir1/e2e.yaml",
		"dir2/subdir2/e2e.yaml",
	}

	for _, file := range configFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test: config"), 0644); err != nil {
			t.Fatalf("failed to write config file %s: %v", file, err)
		}
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// Test finding config files
	walker := e2e.NewFileWalker("e2e.yaml", 0)
	files, err := walker.FindConfigFiles()
	if err != nil {
		t.Fatalf("failed to find config files: %v", err)
	}

	if len(files) != len(configFiles) {
		t.Errorf("expected %d config files, got %d", len(configFiles), len(files))
	}

	// Verify all expected files were found
	found := make(map[string]bool)
	for _, file := range files {
		found[file] = true
	}

	for _, expected := range configFiles {
		if !found[expected] {
			t.Errorf("expected to find config file %s", expected)
		}
	}
}

func TestFileWalkerWithNoConfigFiles(t *testing.T) {
	// Create a temporary directory with no config files
	tmpDir, err := os.MkdirTemp("", "e2e-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	walker := e2e.NewFileWalker("e2e.yaml", 0)
	files, err := walker.FindConfigFiles()
	if err != nil {
		t.Fatalf("failed to find config files: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected no config files, got %d", len(files))
	}
}

func TestFileWalkerWithCustomConfigName(t *testing.T) {
	// Create a temporary directory with a custom config file
	tmpDir, err := os.MkdirTemp("", "e2e-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	customConfig := "custom.yaml"
	if err := os.WriteFile(filepath.Join(tmpDir, customConfig), []byte("test: config"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	walker := e2e.NewFileWalker(customConfig, 0)
	files, err := walker.FindConfigFiles()
	if err != nil {
		t.Fatalf("failed to find config files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 config file, got %d", len(files))
	}

	if files[0] != customConfig {
		t.Errorf("expected to find %s, got %s", customConfig, files[0])
	}
}
