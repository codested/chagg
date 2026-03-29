package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

func CheckCommand() *cli.Command {
	return &cli.Command{
		Name:      "check",
		Aliases:   []string{"c"},
		Usage:     "Validate change entries and show their version attribution",
		ArgsUsage: "[file ...]",
		Description: "Without arguments, validates all change entries in every .changes directory. " +
			"With file arguments (supports glob patterns like *.md), checks only those files " +
			"and shows which release version each entry belongs to, or 'staging' if unreleased.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: text, json",
				Value: "text",
			},
		},
		Action: checkAction,
	}
}

// checkEntry holds the result of validating a single change entry.
type checkEntry struct {
	Status  string   `json:"status"`
	Module  string   `json:"module"`
	Version string   `json:"version"`
	Path    string   `json:"path"`
	Errors  []string `json:"errors,omitempty"`
}

type checkReport struct {
	SchemaVersion int          `json:"schema_version"`
	Entries       []checkEntry `json:"entries"`
	Summary       struct {
		Total   int `json:"total"`
		Valid   int `json:"valid"`
		Invalid int `json:"invalid"`
	} `json:"summary"`
}

func checkAction(_ context.Context, cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, hasGit, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	format := strings.ToLower(strings.TrimSpace(cmd.String("format")))

	args := cmd.Args().Slice()
	if len(args) > 0 {
		return checkFiles(args, repoRoot, hasGit, format)
	}
	return checkAll(cwd, repoRoot, format)
}

func checkAll(cwd, repoRoot, format string) error {
	results, err := changeentry.CheckAllChangesDirs(cwd)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		if format == "json" {
			return renderCheckJSON(nil, os.Stdout)
		}
		fmt.Println("No change entries found.")
		return nil
	}

	// Load version info for all files (best-effort).
	versionByPath, err := loadVersionsForResults(repoRoot, results)
	if err != nil {
		versionByPath = map[string]string{}
	}

	sort.SliceStable(results, func(i, j int) bool {
		moduleI := strings.ToLower(results[i].Module.Name)
		moduleJ := strings.ToLower(results[j].Module.Name)
		if moduleI != moduleJ {
			return moduleI < moduleJ
		}
		return results[i].Path < results[j].Path
	})

	entries := make([]checkEntry, 0, len(results))
	for _, result := range results {
		moduleText := result.Module.Name
		if moduleText == "" {
			moduleText = "default"
		}

		pathText := result.Path
		if relPath, relErr := filepath.Rel(repoRoot, result.Path); relErr == nil {
			pathText = relPath
		}

		version := versionByPath[result.Path]
		if version == "" {
			version = "staging"
		}

		ce := checkEntry{
			Module:  moduleText,
			Version: version,
			Path:    pathText,
		}
		if result.Valid() {
			ce.Status = "ok"
		} else {
			ce.Status = "fail"
			for _, e := range result.Errors {
				ce.Errors = append(ce.Errors, e.Error())
			}
		}
		entries = append(entries, ce)
	}

	if format == "json" {
		return renderCheckJSONWithError(entries, os.Stdout)
	}

	return renderCheckText(entries)
}

func checkFiles(patterns []string, repoRoot string, hasGit bool, format string) error {
	// Expand glob patterns.
	var paths []string
	for _, pattern := range patterns {
		matches, globErr := filepath.Glob(pattern)
		if globErr != nil {
			return fmt.Errorf("invalid pattern %q: %w", pattern, globErr)
		}
		if len(matches) == 0 {
			// Treat as literal path.
			paths = append(paths, pattern)
		} else {
			paths = append(paths, matches...)
		}
	}

	var entries []checkEntry
	for _, path := range paths {
		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			entries = append(entries, checkEntry{
				Status: "fail",
				Path:   path,
				Errors: []string{absErr.Error()},
			})
			continue
		}

		contentBytes, readErr := os.ReadFile(absPath)
		if readErr != nil {
			entries = append(entries, checkEntry{
				Status: "fail",
				Path:   path,
				Errors: []string{readErr.Error()},
			})
			continue
		}

		// Find the module for this file.
		changesDir := findContainingChangesDir(absPath, repoRoot)
		module, modErr := changeentry.ResolveModuleForChangesDir(repoRoot, changesDir)
		if modErr != nil {
			module = changeentry.ModuleConfig{}
		}

		entry, errs := changeentry.ParseEntry(string(contentBytes), absPath, module)

		// Resolve version.
		version := "staging"
		if hasGit {
			tags, tagsErr := gitutil.ListSemVerTags(repoRoot, module.TagPrefix)
			if tagsErr == nil {
				addedAt, _, _, addedOk := gitutil.FileAddedMeta(repoRoot, absPath)
				version = changelog.ResolveVersion(entry, addedAt, addedOk, tags)
			}
		}

		relPath := path
		if rel, relErr := filepath.Rel(repoRoot, absPath); relErr == nil {
			relPath = rel
		}

		moduleText := module.Name
		if moduleText == "" {
			moduleText = "default"
		}

		ce := checkEntry{
			Module:  moduleText,
			Version: version,
			Path:    relPath,
		}
		if len(errs) == 0 {
			ce.Status = "ok"
		} else {
			ce.Status = "fail"
			for _, e := range errs {
				ce.Errors = append(ce.Errors, e.Error())
			}
		}
		entries = append(entries, ce)
	}

	if format == "json" {
		return renderCheckJSONWithError(entries, os.Stdout)
	}

	return renderCheckText(entries)
}

func renderCheckText(entries []checkEntry) error {
	var validCount, invalidCount int
	for _, ce := range entries {
		if ce.Status == "ok" {
			validCount++
			fmt.Printf("  ok  [%s] [%s] %s\n", ce.Module, ce.Version, ce.Path)
		} else {
			invalidCount++
			fmt.Printf("FAIL  [%s] [%s] %s\n", ce.Module, ce.Version, ce.Path)
			for _, e := range ce.Errors {
				fmt.Printf("      %s\n", e)
			}
		}
	}

	total := validCount + invalidCount
	fmt.Println()
	fmt.Printf("Checked %d change %s: %d valid, %d invalid.\n",
		total, pluralise(total, "entry", "entries"), validCount, invalidCount)

	if invalidCount > 0 {
		return changeentry.NewValidationError("", fmt.Sprintf("%d of %d change %s invalid",
			invalidCount, total, pluralise(total, "entry is", "entries are")))
	}
	fmt.Printf("All %d change %s valid.\n", total, pluralise(total, "entry is", "entries are"))
	return nil
}

func renderCheckJSON(entries []checkEntry, w *os.File) error {
	return renderCheckJSONWithError(entries, w)
}

func renderCheckJSONWithError(entries []checkEntry, w *os.File) error {
	if entries == nil {
		entries = []checkEntry{}
	}
	report := checkReport{SchemaVersion: 1, Entries: entries}
	var invalidCount int
	for _, e := range entries {
		if e.Status == "ok" {
			report.Summary.Valid++
		} else {
			report.Summary.Invalid++
			invalidCount++
		}
	}
	report.Summary.Total = len(entries)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}

	if invalidCount > 0 {
		return changeentry.NewValidationError("", fmt.Sprintf("%d of %d change %s invalid",
			invalidCount, len(entries), pluralise(len(entries), "entry is", "entries are")))
	}
	return nil
}

// findContainingChangesDir finds the .changes directory that contains the given file.
// Falls back to the repo root .changes if not found.
func findContainingChangesDir(absPath, repoRoot string) string {
	dir := filepath.Dir(absPath)
	for {
		if filepath.Base(dir) == ".changes" {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(dir, repoRoot) {
			break
		}
		dir = parent
	}
	return filepath.Join(repoRoot, ".changes")
}

// loadVersionsForResults loads version info for all check results using git.
func loadVersionsForResults(repoRoot string, results []changeentry.CheckResult) (map[string]string, error) {
	if len(results) == 0 {
		return map[string]string{}, nil
	}

	// Group by module to minimise git tag queries.
	type moduleKey struct {
		tagPrefix  string
		changesDir string
	}
	moduleMap := map[moduleKey][]changeentry.CheckResult{}
	for _, r := range results {
		key := moduleKey{tagPrefix: r.Module.TagPrefix, changesDir: r.Module.ChangesDir}
		moduleMap[key] = append(moduleMap[key], r)
	}

	versionByPath := map[string]string{}
	for key, moduleResults := range moduleMap {
		tags, err := gitutil.ListSemVerTags(repoRoot, key.tagPrefix)
		if err != nil {
			continue
		}

		module := moduleResults[0].Module

		for _, result := range moduleResults {
			contentBytes, readErr := os.ReadFile(result.Path)
			if readErr != nil {
				continue
			}
			entry, errs := changeentry.ParseEntry(string(contentBytes), result.Path, module)
			if len(errs) > 0 {
				continue
			}
			addedAt, _, _, addedOk := gitutil.FileAddedMeta(repoRoot, result.Path)
			versionByPath[result.Path] = changelog.ResolveVersion(entry, addedAt, addedOk, tags)
		}
	}

	return versionByPath, nil
}

func pluralise(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// moduleClause returns " for module \"<name>\"" when the module has a name,
// or an empty string for the root module (name == ""). This avoids printing
// 'for module ""' in single-module repositories.
func moduleClause(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf(" for module %q", name)
}
