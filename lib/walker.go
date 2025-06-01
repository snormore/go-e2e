package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileWalker is a helper to find files in the current directory and its subdirectories.
// It uses git to check if a file is ignored by the .gitignore file.
// It also skips hidden files and directories.
type FileWalker struct {
	fileName  string
	verbosity int
}

func NewFileWalker(fileName string, verbosity int) *FileWalker {
	return &FileWalker{
		fileName:  fileName,
		verbosity: verbosity,
	}
}

func (w *FileWalker) FindConfigFiles() ([]string, error) {
	var configFiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden files/dirs except the root directory
		if strings.HasPrefix(info.Name(), ".") && path != "." {
			if w.verbosity > 2 {
				fmt.Printf("--- INFO: Ignoring hidden file or directory: %s\n", path)
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		cmd := exec.Command("git", "check-ignore", "-q", path)
		if err := cmd.Run(); err == nil {
			if info.IsDir() {
				if w.verbosity > 2 {
					fmt.Printf("--- INFO: Ignoring directory %s because it matches a .gitignore entry\n", path)
				}
				return filepath.SkipDir
			}
			if w.verbosity > 2 {
				fmt.Printf("--- INFO: Ignoring file %s because it matches a .gitignore entry\n", path)
			}
			return nil
		}

		if !info.IsDir() && info.Name() == filepath.Base(w.fileName) {
			if w.verbosity > 2 {
				fmt.Printf("--- INFO: Found e2e.yaml file: %s\n", path)
			}
			configFiles = append(configFiles, path)
		}
		return nil
	})
	return configFiles, err
}
