package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/urfave/cli/v3"
)

func CheckCommand() *cli.Command {
	return &cli.Command{
		Name:   "check",
		Usage:  "Validate all change entries in every .changes directory",
		Action: checkAction,
	}
}

func checkAction(ctx context.Context, cmd *cli.Command) error {
	_ = ctx
	_ = cmd

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	results, err := changeentry.CheckAllChangesDirs(cwd)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		fmt.Println("No change entries found.")
		return nil
	}

	var invalidCount int
	for _, result := range results {
		moduleText := result.Module.Name
		if moduleText == "" {
			moduleText = "default"
		}

		if result.Valid() {
			fmt.Printf("  ok  [%s] %s\n", moduleText, result.Path)
		} else {
			invalidCount++
			fmt.Printf("FAIL  [%s] %s\n", moduleText, result.Path)
			for _, e := range result.Errors {
				fmt.Printf("      %s\n", e)
			}
		}
	}

	fmt.Println()

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
