package changeentry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codested/chagg/internal/gitutil"
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
	root, _, err := gitutil.FindGitRoot(startPath)
	if err != nil {
		return nil, err
	}

	dirs, err := gitutil.FindAllChangesDirs(root)
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
