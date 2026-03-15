package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
		ArgsUsage: "[path]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: changeentry.DefaultTypeRegistry().TypeFlagUsage()},
			&cli.StringFlag{Name: "bump", Usage: changeentry.BumpFlagUsage()},
			&cli.StringFlag{Name: "component", Usage: "Component(s), comma separated"},
			&cli.StringFlag{Name: "audience", Usage: "Audience(s), comma separated"},
			&cli.IntFlag{Name: "rank", Usage: "Higher rank values are shown first in changelog output"},
			&cli.StringFlag{Name: "issue", Usage: "Issue reference(s), comma separated"},
			&cli.StringFlag{Name: "release", Usage: "Pin entry to release version"},
			&cli.StringFlag{Name: "body", Usage: "Markdown body content"},
			&cli.BoolFlag{Name: "git-add", Usage: "Stage the created change file with git add"},
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

	interactive := isInteractiveStdin() && !cmd.Bool("no-prompt")
	path, err := changeentry.CreateChange(module, cmd.Args().Get(0), params, os.Stdin, os.Stdout, interactive)
	if err != nil {
		return err
	}

	if shouldGitAdd && hasGitRoot {
		if err := gitAddPath(repoRoot, path); err != nil {
			return err
		}
	}

	fmt.Printf("Created %s\n", path)
	if shouldGitAdd && hasGitRoot {
		fmt.Printf("Staged %s\n", path)
	}
	return nil
}

func resolveGitAddBehavior(cmd *cli.Command) (bool, error) {
	if cmd.IsSet("git-add") && cmd.IsSet("no-git-add") {
		if cmd.Bool("git-add") == cmd.Bool("no-git-add") {
			return false, changeentry.NewValidationError("flags", "--git-add and --no-git-add cannot be used together")
		}
	}

	if cmd.IsSet("no-git-add") && cmd.Bool("no-git-add") {
		return false, nil
	}

	if cmd.IsSet("git-add") {
		return cmd.Bool("git-add"), nil
	}

	return true, nil
}

func gitAddPath(repoRoot string, path string) error {
	relPath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		relPath = path
	}

	cmd := exec.Command("git", "add", "--", relPath)
	cmd.Dir = repoRoot
	if out, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = cmdErr.Error()
		}
		return fmt.Errorf("git add %s: %s", relPath, message)
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
