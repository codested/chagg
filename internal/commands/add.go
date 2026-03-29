package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Aliases:   []string{"a"},
		Usage:     "Create a new change entry",
		ArgsUsage: "<unique-slug>",
		Description: "Creates a new change entry file under .changes/.\n\n" +
			"The required <unique-slug> argument is a short, descriptive identifier for this\n" +
			"specific change — not a type, category, or component name. It becomes part of\n" +
			"the filename, so it must uniquely identify the change.\n\n" +
			"You can embed the type directly in the slug using the '<type>__<description>'\n" +
			"filename convention, which makes --type optional:\n\n" +
			"  chagg add feat__oauth-login             # type=feature inferred from prefix\n" +
			"  chagg add fix__token-expiry             # type=fix inferred from prefix\n" +
			"  chagg add auth/fix__session-timeout     # subdirectory + inferred type\n\n" +
			"Without an embedded type, pass --type explicitly:\n\n" +
			"  chagg add auth/token-expiry --type fix\n" +
			"  chagg add docs/release-notes --type docs\n\n" +
			"Bad examples (slug must describe the change, not just a type or area):\n\n" +
			"  chagg add web          # BAD: not descriptive\n" +
			"  chagg add feat         # BAD: slug is the type itself, not a description\n" +
			"  chagg add api          # BAD: this is a component, not a change description\n\n" +
			"Use --no-prompt for CI and AI tooling to avoid interactive prompts.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: changeentry.DefaultTypeRegistry().TypeFlagUsage()},
			&cli.StringFlag{Name: "bump", Usage: changeentry.BumpFlagUsage()},
			&cli.StringFlag{Name: "component", Usage: "Component(s), comma separated"},
			&cli.StringFlag{Name: "audience", Usage: "Audience(s), comma separated"},
			&cli.IntFlag{Name: "rank", Usage: "Higher rank values are shown first in changelog output"},
			&cli.StringFlag{Name: "issue", Usage: "Issue reference(s), comma separated"},
			&cli.StringFlag{Name: "release", Usage: "Pin entry to release version"},
			&cli.StringFlag{Name: "body", Usage: "Markdown body content"},
			&cli.BoolFlag{Name: "no-git-add", Usage: "Do not stage the created change file"},
			&cli.BoolFlag{Name: "no-prompt", Usage: "Disable interactive prompts and use defaults for omitted fields"},
		},
		Action: addAction,
	}
}

func addAction(_ context.Context, cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, hasGitRoot, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	changesDir, err := changeentry.ResolveChangesDirectory(cwd)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(changesDir); os.IsNotExist(statErr) {
		return changeentry.NewValidationError("init",
			"no .changes directory found; run 'chagg init' to set up this repository")
	}

	module, err := changeentry.ResolveModuleForChangesDir(repoRoot, changesDir)
	if err != nil {
		return err
	}

	shouldGitAdd, err := resolveGitAddBehavior(cmd)
	if err != nil {
		return err
	}
	if shouldGitAdd && !module.GitWrite.AllowsAdd() {
		return changeentry.NewValidationError("config", "git add is disabled by git-write policy")
	}

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		return fmt.Errorf("create changes directory: %w", err)
	}

	params := changeentry.Params{
		Type:         cmd.String("type"),
		TypeSet:      cmd.IsSet("type"),
		Bump:         cmd.String("bump"),
		BumpSet:      cmd.IsSet("bump"),
		Component:    cmd.String("component"),
		ComponentSet: cmd.IsSet("component"),
		Audience:     cmd.String("audience"),
		AudienceSet:  cmd.IsSet("audience"),
		Rank:         cmd.Int("rank"),
		RankSet:      cmd.IsSet("rank"),
		Issue:        cmd.String("issue"),
		IssueSet:     cmd.IsSet("issue"),
		Release:      cmd.String("release"),
		ReleaseSet:   cmd.IsSet("release"),
		Body:         cmd.String("body"),
		BodySet:      cmd.IsSet("body"),
		Defaults:     module.Defaults,
	}

	// --body - reads body from stdin.
	stdinConsumed := false
	if params.BodySet && params.Body == "-" {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return fmt.Errorf("read body from stdin: %w", readErr)
		}
		params.Body = strings.TrimRight(string(data), "\n")
		stdinConsumed = true
	}

	interactive := !stdinConsumed && isInteractiveStdin() && !cmd.Bool("no-prompt")
	path, err := changeentry.CreateChange(module, cmd.Args().Get(0), params, os.Stdin, os.Stdout, interactive)
	if err != nil {
		return err
	}

	if shouldGitAdd && hasGitRoot {
		if err := gitAddPath(repoRoot, path); err != nil {
			return err
		}
	}

	if !cmd.Root().Bool("quiet") {
		fmt.Printf("Created %s\n", path)
		if shouldGitAdd && hasGitRoot {
			fmt.Printf("Staged %s\n", path)
		}
	}
	return nil
}

func resolveGitAddBehavior(cmd *cli.Command) (bool, error) {
	if cmd.IsSet("no-git-add") && cmd.Bool("no-git-add") {
		return false, nil
	}
	return true, nil
}

func gitAddPath(repoRoot string, path string) error {
	relPath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		relPath = path
	}

	if _, err := gitutil.RunGit(repoRoot, "add", "--", relPath); err != nil {
		return fmt.Errorf("git add %s: %w", relPath, err)
	}

	return nil
}

func isInteractiveStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}
