package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

func CheckCommand() *cli.Command {
	return &cli.Command{
		Name:    "check",
		Aliases: []string{"c"},
		Usage:   "Validate all change entries in every .changes directory",
		Action:  checkAction,
	}
}

func checkAction(_ context.Context, _ *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	results, err := changeentry.CheckAllChangesDirs(cwd)
	if err != nil {
		return err
	}

	repoRoot, _, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No change entries found.")
		return nil
	}

	sort.SliceStable(results, func(i, j int) bool {
		moduleI := strings.ToLower(results[i].Module.Name)
		moduleJ := strings.ToLower(results[j].Module.Name)
		if moduleI != moduleJ {
			return moduleI < moduleJ
		}
		return results[i].Path < results[j].Path
	})

	var invalidCount int
	var validCount int
	for _, result := range results {
		moduleText := result.Module.Name
		if moduleText == "" {
			moduleText = "default"
		}

		pathText := result.Path
		relPath, relErr := filepath.Rel(repoRoot, result.Path)
		if relErr == nil {
			pathText = relPath
		}

		if result.Valid() {
			validCount++
			fmt.Printf("  ok  [%s] %s\n", moduleText, pathText)
		} else {
			invalidCount++
			fmt.Printf("FAIL  [%s] %s\n", moduleText, pathText)
			for _, e := range result.Errors {
				fmt.Printf("      %s\n", e)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Checked %d change %s: %d valid, %d invalid.\n",
		len(results), pluralise(len(results), "entry", "entries"), validCount, invalidCount)

	if invalidCount > 0 {
		return changeentry.NewValidationError("", fmt.Sprintf("%d of %d change %s invalid",
			invalidCount, len(results), pluralise(len(results), "entry is", "entries are")))
	}

	fmt.Printf("All %d change %s valid.\n", len(results), pluralise(len(results), "entry is", "entries are"))
	return nil
}

func pluralise(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
