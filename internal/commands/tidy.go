package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/urfave/cli/v3"
)

type tidyMove struct {
	ModuleName string
	ChangesDir string
	From       string
	To         string
}

func TidyCommand() *cli.Command {
	return &cli.Command{
		Name:  "tidy",
		Usage: "Reorganize released change entries without creating commits",
		Description: "Moves released entries into .changes/archive/<version>/ while leaving staging entries in place. " +
			"Runs in dry-run mode by default.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "apply",
				Usage: "Apply planned file moves",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Process all .changes directories under the repository root",
			},
		},
		Action: tidyAction,
	}
}

func tidyAction(ctx context.Context, cmd *cli.Command) error {
	_ = ctx

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, _, err := changeentry.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	changesDirs, err := resolveTidyChangesDirs(cwd, repoRoot, cmd.Bool("all"))
	if err != nil {
		return err
	}

	moves, err := planTidyMoves(repoRoot, changesDirs)
	if err != nil {
		return err
	}

	if len(moves) == 0 {
		fmt.Println("Nothing to tidy.")
		return nil
	}

	if !cmd.Bool("apply") {
		fmt.Printf("Tidy plan (%d move%s):\n", len(moves), pluralize(len(moves), "", "s"))
		for _, move := range moves {
			fmt.Printf("  [%s] %s -> %s\n", move.ModuleName, move.From, move.To)
		}
		fmt.Println()
		fmt.Println("Dry-run only. Re-run with --apply to execute these moves.")
		return nil
	}

	touchedChangesDirs := map[string]bool{}
	for _, move := range moves {
		touchedChangesDirs[move.ChangesDir] = true
		if err := applyTidyMove(move); err != nil {
			return err
		}
		fmt.Printf("Moved [%s] %s -> %s\n", move.ModuleName, move.From, move.To)
	}

	for changesDir := range touchedChangesDirs {
		if err := pruneEmptyChangeDirectories(changesDir); err != nil {
			return err
		}
	}

	fmt.Printf("Done. Applied %d move%s.\n", len(moves), pluralize(len(moves), "", "s"))
	return nil
}

func resolveTidyChangesDirs(cwd, repoRoot string, all bool) ([]string, error) {
	if !all {
		dir, err := changeentry.ResolveChangesDirectory(cwd)
		if err != nil {
			return nil, err
		}
		return []string{dir}, nil
	}

	dirs, err := changeentry.FindAllChangesDirs(repoRoot)
	if err != nil {
		return nil, err
	}

	return dirs, nil
}

func planTidyMoves(repoRoot string, changesDirs []string) ([]tidyMove, error) {
	moves := make([]tidyMove, 0)
	targetOwners := map[string]string{}
	for _, changesDir := range changesDirs {
		module, err := changeentry.ResolveModuleForChangesDir(repoRoot, changesDir)
		if err != nil {
			return nil, err
		}

		cl, err := changelog.LoadChangeLog(repoRoot, module, changelog.FilterOptions{})
		if err != nil {
			return nil, err
		}

		for _, group := range cl.Groups {
			if group.IsStaging() || group.Tag == nil {
				continue
			}

			versionDir := sanitizeVersionDirName(group.Version)
			for _, tg := range group.TypeGroups {
				for _, entry := range tg.Entries {
					if isAlreadyArchivedInVersion(module.ChangesDir, versionDir, entry.Path) {
						continue
					}

					to := archiveTargetPath(module.ChangesDir, versionDir, entry.Path)
					if entry.Path == to {
						continue
					}

					if owner, exists := targetOwners[to]; exists && owner != entry.Path {
						return nil, changeentry.NewConflictError(fmt.Sprintf("tidy target collision: %s and %s both map to %s", owner, entry.Path, to))
					}
					targetOwners[to] = entry.Path

					moves = append(moves, tidyMove{
						ModuleName: module.Name,
						ChangesDir: module.ChangesDir,
						From:       entry.Path,
						To:         to,
					})
				}
			}
		}
	}

	sort.SliceStable(moves, func(i, j int) bool {
		if moves[i].ModuleName != moves[j].ModuleName {
			return moves[i].ModuleName < moves[j].ModuleName
		}
		return moves[i].From < moves[j].From
	})

	return dedupeTidyMoves(moves), nil
}

func dedupeTidyMoves(moves []tidyMove) []tidyMove {
	seen := map[string]bool{}
	result := make([]tidyMove, 0, len(moves))
	for _, move := range moves {
		key := move.From + "->" + move.To
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, move)
	}

	return result
}

func applyTidyMove(move tidyMove) error {
	if _, err := os.Stat(move.From); err != nil {
		return fmt.Errorf("source file not found %s: %w", move.From, err)
	}

	if _, err := os.Stat(move.To); err == nil {
		return changeentry.NewConflictError(fmt.Sprintf("target file already exists: %s", move.To))
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(move.To), 0o755); err != nil {
		return err
	}

	if err := os.Rename(move.From, move.To); err != nil {
		return err
	}

	return nil
}

func pruneEmptyChangeDirectories(changesDir string) error {
	dirs := make([]string, 0)
	err := filepath.WalkDir(changesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == changesDir {
			return nil
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return err
	}

	sort.SliceStable(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, dir := range dirs {
		removeErr := os.Remove(dir)
		if removeErr == nil || os.IsNotExist(removeErr) {
			continue
		}
		if pe, ok := removeErr.(*os.PathError); ok {
			message := strings.ToLower(pe.Err.Error())
			if strings.Contains(message, "directory not empty") || strings.Contains(message, "not empty") {
				continue
			}
		}
		return removeErr
	}

	return nil
}

func sanitizeVersionDirName(version string) string {
	s := strings.TrimSpace(version)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, string(filepath.Separator), "_")
	if s == "" {
		return "unknown"
	}

	return s
}

func archiveTargetPath(changesDir string, versionDir string, sourcePath string) string {
	return filepath.Join(changesDir, "archive", versionDir, filepath.Base(sourcePath))
}

func isAlreadyArchivedInVersion(changesDir string, versionDir string, path string) bool {
	archiveDir := filepath.Join(changesDir, "archive", versionDir)
	relPath, err := filepath.Rel(archiveDir, path)
	if err != nil {
		return false
	}

	if relPath == "." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) || relPath == ".." {
		return false
	}

	return true
}
