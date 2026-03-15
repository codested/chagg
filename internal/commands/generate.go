package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

func GenerateCommand() *cli.Command {
	return &cli.Command{
		Name:    "generate",
		Aliases: []string{"gen", "g"},
		Usage:   "Generate a changelog from all change entries",
		Description: "Produces a full changelog grouped by version and change type. " +
			"By default shows staging changes and the most recent tagged release (-n 1 --show-staging). " +
			"Use -n to control how many tagged releases to include (0 = all), " +
			"--only-staging for staging-only output, " +
			"--no-show-staging to omit unreleased changes, " +
			"--since to set a version boundary, " +
			"and --audience / --component / --type to filter entries.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: markdown, json",
				Value: "markdown",
			},
			&cli.IntFlag{
				Name:  "n",
				Usage: "Number of tagged releases to include, newest first (0 = all)",
				Value: 1,
			},
			&cli.BoolFlag{
				Name:  "show-staging",
				Usage: "Include unreleased (staging) changes",
				Value: true,
			},
			&cli.BoolFlag{
				Name:  "only-staging",
				Usage: "Include only unreleased (staging) changes",
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
				Usage: changeentry.DefaultTypeRegistry().TypeFlagUsage(),
			},
		},
		Action: generateAction,
	}
}

func generateAction(_ context.Context, cmd *cli.Command) error {
	if err := validateGenerateFlags(cmd); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, _, err := gitutil.FindGitRoot(cwd)
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

	cl = changelog.ApplyVersionFilter(cl, changelog.VersionFilterOptions{
		N:           cmd.Int("n"),
		ShowStaging: cmd.Bool("show-staging"),
		OnlyStaging: cmd.Bool("only-staging"),
		Since:       cmd.String("since"),
	})

	format := normalizeGenerateFormat(cmd.String("format"))
	switch format {
	case "markdown":
		return changelog.RenderMarkdown(cl, os.Stdout)
	case "json":
		return changelog.RenderJSON(cl, repoRoot, os.Stdout)
	default:
		return changeentry.NewValidationError("format", fmt.Sprintf("unsupported format %q (use markdown or json)", cmd.String("format")))
	}
}

func validateGenerateFlags(cmd *cli.Command) error {
	if !cmd.Bool("only-staging") {
		return nil
	}

	if strings.TrimSpace(cmd.String("since")) != "" {
		return changeentry.NewValidationError("flags", "--only-staging cannot be combined with --since")
	}
	if cmd.IsSet("n") {
		return changeentry.NewValidationError("flags", "--only-staging cannot be combined with -n")
	}
	if !cmd.Bool("show-staging") {
		return changeentry.NewValidationError("flags", "--only-staging cannot be combined with --no-show-staging")
	}

	return nil
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
