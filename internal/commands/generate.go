package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/urfave/cli/v3"
)

func GenerateCommand() *cli.Command {
	return &cli.Command{
		Name:    "generate",
		Aliases: []string{"gen", "g"},
		Usage:   "Generate a changelog from all change entries",
		Description: "Produces a full changelog grouped by version and change type. " +
			"Use --latest or --since to restrict the version range, " +
			"and --audience / --component / --type to filter entries.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: markdown, json",
				Value: "markdown",
			},
			&cli.BoolFlag{
				Name:  "latest",
				Usage: "Include only the most recent tagged release",
			},
			&cli.StringFlag{
				Name:  "since",
				Usage: "Include this version and all newer versions (e.g. v1.2.0)",
			},
			&cli.StringFlag{
				Name:  "audience",
				Usage: "Include only entries for this audience (e.g. public)",
			},
			&cli.StringFlag{
				Name:  "component",
				Usage: "Include only entries for this component",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: changeentry.TypeFlagUsage(),
			},
		},
		Action: generateAction,
	}
}

func generateAction(_ context.Context, cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, _, err := changeentry.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	changesDir, err := changeentry.ResolveChangesDirectory(cwd)
	if err != nil {
		return err
	}

	module, err := changeentry.ResolveModuleForChangesDir(repoRoot, changesDir)
	if err != nil {
		return err
	}

	filter := changelog.FilterOptions{
		Audience:  cmd.String("audience"),
		Component: cmd.String("component"),
		Type:      cmd.String("type"),
	}

	cl, loadErr := changelog.LoadChangeLog(repoRoot, module, filter)
	if loadErr != nil {
		return loadErr
	}

	cl = changelog.ApplyVersionFilter(cl, cmd.String("since"), cmd.Bool("latest"))

	format := normalizeGenerateFormat(cmd.String("format"))
	switch format {
	case "markdown":
		return changelog.RenderMarkdown(cl, os.Stdout)
	case "json":
		return changelog.RenderJSON(cl, os.Stdout)
	default:
		return changeentry.NewValidationError("format", fmt.Sprintf("unsupported format %q (use markdown or json)", cmd.String("format")))
	}
}

func normalizeGenerateFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "md":
		return "markdown"
	default:
		return format
	}
}
