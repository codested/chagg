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
				Name:  "audience",
				Usage: "Show only entries for this audience (e.g. public)",
			},
			&cli.StringFlag{
				Name:  "component",
				Usage: "Show only entries for this component",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: changeentry.TypeFlagUsage(),
			},
		},
		Action: logAction,
	}
}

func logAction(ctx context.Context, cmd *cli.Command) error {
	_ = ctx

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

	version := strings.TrimSpace(cmd.Args().Get(0))

	var view *changelog.ChangeLog
	if version == "" || strings.EqualFold(version, "staging") {
		view = changelog.StagingOnly(cl)
	} else {
		view = changelog.VersionOnly(cl, version)
	}

	return changelog.RenderLog(view, repoRoot, os.Stdout)
}
