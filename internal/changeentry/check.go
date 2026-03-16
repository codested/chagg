package changeentry

import (
	"fmt"
	"log/slog"
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
// Files that resolve (via symlinks) to a path outside changesDir are skipped with a warning.
func CheckChangesDir(changesDir string, module ModuleConfig) ([]CheckResult, error) {
	absChangesDir, err := filepath.Abs(changesDir)
	if err != nil {
		return nil, err
	}
	// Resolve the changesDir itself so comparisons work on macOS where /var → /private/var.
	realChangesDir := absChangesDir
	if resolved, resolveErr := filepath.EvalSymlinks(absChangesDir); resolveErr == nil {
		realChangesDir = resolved
	}
	slog.Debug("checking changes dir", "dir", realChangesDir, "module", module.Name)

	var results []CheckResult

	walkErr := filepath.WalkDir(absChangesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}

		// Resolve symlinks and verify the real path stays within changesDir.
		realPath, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			slog.Warn("skipping unresolvable symlink", "path", path, "err", resolveErr)
			return nil
		}
		if !gitutil.IsWithinDir(realChangesDir, realPath) {
			slog.Warn("skipping entry whose symlink target escapes .changes", "path", path, "target", realPath)
			return nil
		}

		slog.Debug("checking entry", "path", path)
		contentBytes, readErr := os.ReadFile(realPath)
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

	slog.Info("checked changes dir", "dir", absChangesDir, "entries", len(results))
	return results, walkErr
}

// CheckAllChangesDirs locates every ".changes" directory reachable from startPath
// (by first finding the git root) and validates all change entries inside them.
func CheckAllChangesDirs(startPath string) ([]CheckResult, error) {
	root, _, err := gitutil.FindGitRoot(startPath)
	if err != nil {
		return nil, err
	}
	slog.Info("checking all .changes dirs", "root", root)

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
