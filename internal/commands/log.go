package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/codested/chagg/internal/semver"
	"github.com/urfave/cli/v3"
)

func LogCommand() *cli.Command {
	return &cli.Command{
		Name:      "log",
		Aliases:   []string{"l"},
		Usage:     "List change entries",
		ArgsUsage: "[version]",
		Description: "Lists staging changes (since the last release) by default. " +
			"Pass a version tag (e.g. v1.2.3) to inspect a specific release.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: text, json",
				Value: "text",
			},
			&cli.BoolFlag{
				Name:  "version-hints",
				Usage: "Show latest and next calculated release tag hints for staging view",
				Value: true,
			},
			&cli.StringFlag{
				Name:  "audience",
				Usage: "Show only entries for this audience (e.g. public)",
			},
			&cli.StringFlag{
				Name:  "component",
				Usage: "Show only entries for this component",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: changeentry.DefaultTypeRegistry().TypeFlagUsage(),
			},
			&cli.IntFlag{
				Name:  "preview-length",
				Usage: "Maximum preview length for each log entry message",
				Value: 80,
			},
		},
		Action: logAction,
	}
}

func logAction(_ context.Context, cmd *cli.Command) error {
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

	version := strings.TrimSpace(cmd.Args().Get(0))

	isStaging := version == "" || strings.EqualFold(version, "staging")
	var view *changelog.ChangeLog
	if isStaging {
		view = changelog.StagingOnly(cl)
	} else {
		view = changelog.VersionOnly(cl, version)
	}

	format := strings.ToLower(strings.TrimSpace(cmd.String("format")))
	switch format {
	case "json":
		var hints *changelog.LogHints
		if isStaging {
			tags, _ := gitutil.ListSemVerTags(repoRoot, module.TagPrefix)
			latestText, nextText := computeVersionHints(module, tags, cl)
			hints = &changelog.LogHints{LatestTag: latestText, NextTag: nextText}
		}
		return changelog.RenderLogJSON(view, repoRoot, hints, os.Stdout)
	case "text", "":
		if isStaging && cmd.Bool("version-hints") {
			if err := renderLogVersionHints(repoRoot, module, cl, os.Stdout); err != nil {
				return err
			}
		}
		return changelog.RenderLog(view, repoRoot, cmd.Int("preview-length"), os.Stdout)
	default:
		return changeentry.NewValidationError("format", fmt.Sprintf("unsupported format %q (use text or json)", cmd.String("format")))
	}
}

func renderLogVersionHints(repoRoot string, module changeentry.ModuleConfig, cl *changelog.ChangeLog, w io.Writer) error {
	tags, _ := gitutil.ListSemVerTags(repoRoot, module.TagPrefix)
	latestText, nextText := computeVersionHints(module, tags, cl)

	_, _ = fmt.Fprintf(w, "Latest stable tag: %s\n", latestText)
	_, _ = fmt.Fprintf(w, "Next calculated tag: %s\n\n", nextText)
	return nil
}

func computeVersionHints(module changeentry.ModuleConfig, tags []semver.Tag, cl *changelog.ChangeLog) (string, string) {
	latestTag, hasLatest := semver.LatestStable(tags)

	latestText := "none"
	if hasLatest {
		latestText = latestTag.Name
	}

	nextText := "none (no staging changes)"
	staging := changelog.StagingOnly(cl)
	if len(staging.Groups) > 0 && staging.Groups[0].TotalEntries() > 0 {
		if !hasLatest {
			nextText = module.TagPrefix + "0.1.0"
		} else {
			next := semver.Bump(latestTag.Version, detectBumpLevel(staging.Groups[0], module.Types))
			nextText = module.TagPrefix + next.String(latestTag.HasVPrefix)
		}
	}

	return latestText, nextText
}
