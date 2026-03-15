package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CheckResult holds the validation outcome for a single change entry file.
type CheckResult struct {
	Module ModuleConfig
	Path   string
	Errors []error
}

// Valid reports whether the entry has no validation errors.
func (r CheckResult) Valid() bool {
	return len(r.Errors) == 0
}

// FindGitRoot walks up from startPath until it finds a directory that contains a
// ".git" entry. It returns the root path and true when found. If the filesystem
// root is reached without finding ".git", it returns startPath and false.
func FindGitRoot(startPath string) (string, bool, error) {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return "", false, err
	}

	for {
		gitPath := filepath.Join(current, ".git")
		if _, gitErr := os.Stat(gitPath); gitErr == nil {
			return current, true, nil
		} else if !errors.Is(gitErr, os.ErrNotExist) {
			return "", false, gitErr
		}

		parent := filepath.Dir(current)
		if parent == current {
			return current, false, nil
		}
		current = parent
	}
}

// FindAllChangesDirs recursively finds all ".changes" directories under root.
// It does not recurse into ".git" directories or into ".changes" directories
// themselves (nested ".changes" hierarchies are not supported).
func FindAllChangesDirs(root string) ([]string, error) {
	var dirs []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		if name == ".changes" {
			dirs = append(dirs, path)
			return filepath.SkipDir
		}

		// Skip .git and other hidden directories (they will never contain .changes).
		if name != "." && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		return nil
	})

	return dirs, err
}

// CheckChangesDir validates all ".md" files found recursively inside changesDir.
// Each file is parsed and its validation errors are collected in CheckResult.
func CheckChangesDir(changesDir string, module ModuleConfig) ([]CheckResult, error) {
	var results []CheckResult

	err := filepath.WalkDir(changesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			results = append(results, CheckResult{
				Path:   path,
				Errors: []error{fmt.Errorf("read file: %w", readErr)},
			})
			return nil
		}

		_, errs := ParseEntry(string(contentBytes), path, module)
		results = append(results, CheckResult{Path: path, Errors: errs})
		return nil
	})

	return results, err
}

// CheckAllChangesDirs locates every ".changes" directory reachable from startPath
// (by first finding the git root) and validates all change entries inside them.
func CheckAllChangesDirs(startPath string) ([]CheckResult, error) {
	root, _, err := FindGitRoot(startPath)
	if err != nil {
		return nil, err
	}

	dirs, err := FindAllChangesDirs(root)
	if err != nil {
		return nil, err
	}

	modulesByDir, err := ResolveModulesForChangesDirs(root, dirs)
	if err != nil {
		return nil, err
	}

	var allResults []CheckResult
	for _, dir := range dirs {
		module := modulesByDir[dir]
		results, dirErr := CheckChangesDir(dir, module)
		if dirErr != nil {
			return nil, dirErr
		}

		for i := range results {
			results[i].Module = module
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}
