package gitutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/codested/chagg/internal/semver"
)

const defaultGitTimeout = 30 * time.Second

// IsWithinDir reports whether target is inside (or equal to) parent.
// Both paths must be absolute and are cleaned before comparison.
func IsWithinDir(parent, target string) bool {
	p := filepath.Clean(parent)
	t := filepath.Clean(target)
	if p == t {
		return true
	}
	rel, err := filepath.Rel(p, t)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// RunGit runs a git command in dir with a default timeout and returns stdout.
func RunGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer cancel()
	return RunGitWithContext(ctx, dir, args...)
}

// RunGitWithContext runs a git command in dir using the provided context for
// cancellation and deadline control.
func RunGitWithContext(ctx context.Context, dir string, args ...string) (string, error) {
	slog.Debug("git", "args", args, "dir", dir)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git %s: timed out after %s", strings.Join(args, " "), defaultGitTimeout)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// SplitLines splits s into lines, returning nil when s is blank.
func SplitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// ListSemVerTags returns every SemVer tag in the repository at repoRoot,
// optionally scoped by tagPrefix, ordered from oldest to newest by SemVer.
// When git is unavailable or the repository has no matching tags, an empty
// slice and no error are returned.
//
// Tag name and date are fetched in a single git call using --format so the
// number of subprocess invocations is O(1) instead of O(n tags).
// %(creatordate) gives the commit author date for lightweight tags and the
// tagger date for annotated tags; both are accurate enough for version
// attribution ordering.
func ListSemVerTags(repoRoot string, tagPrefix string) ([]semver.Tag, error) {
	slog.Debug("listing semver tags", "repoRoot", repoRoot, "tagPrefix", tagPrefix)

	// Fetch name and ISO-8601 date in one call, separated by a NUL-safe delimiter.
	raw, err := RunGit(repoRoot, "tag", "-l", "--format=%(refname:short)\t%(creatordate:iso-strict)")
	if err != nil {
		slog.Debug("git tag list unavailable, proceeding without history")
		return nil, nil // git unavailable — caller proceeds without history
	}

	var tags []semver.Tag
	for _, line := range SplitLines(raw) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		dateStr := strings.TrimSpace(parts[1])
		if name == "" {
			continue
		}

		if tagPrefix != "" && !strings.HasPrefix(name, tagPrefix) {
			continue
		}

		versionPart := strings.TrimPrefix(name, tagPrefix)
		version, hasVPrefix, parseErr := semver.ParseSemVersion(versionPart)
		if parseErr != nil {
			continue
		}

		t, parseErr := time.Parse(time.RFC3339, dateStr)
		if parseErr != nil {
			return nil, fmt.Errorf("parse date for tag %q: %w", name, parseErr)
		}

		tags = append(tags, semver.Tag{Name: name, CommitDate: t, Version: version, HasVPrefix: hasVPrefix})
	}

	sort.SliceStable(tags, func(i, j int) bool {
		cmp := semver.CompareSemVersion(tags[i].Version, tags[j].Version)
		if cmp != 0 {
			return cmp < 0
		}
		return tags[i].CommitDate.Before(tags[j].CommitDate)
	})

	slog.Info("found semver tags", "count", len(tags), "tagPrefix", tagPrefix)
	return tags, nil
}

// HasAbbreviatedVersionTags reports whether the repository at repoRoot
// contains any abbreviated major (e.g. v1) or major.minor (e.g. v1.2) tags
// with the given prefix. Used to auto-detect the alias-tags release policy.
func HasAbbreviatedVersionTags(repoRoot, tagPrefix string) (bool, error) {
	raw, err := RunGit(repoRoot, "tag", "-l")
	if err != nil {
		return false, nil
	}
	for _, line := range SplitLines(raw) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if tagPrefix != "" && !strings.HasPrefix(name, tagPrefix) {
			continue
		}
		versionPart := strings.TrimPrefix(name, tagPrefix)
		s := strings.TrimPrefix(versionPart, "v")
		s = strings.TrimPrefix(s, "V")
		if len(s) == 0 || s[0] < '0' || s[0] > '9' {
			continue
		}
		// Strip pre-release/build suffix before counting dots
		if idx := strings.IndexAny(s, "-+"); idx >= 0 {
			s = s[:idx]
		}
		if strings.Count(s, ".") < 2 {
			return true, nil
		}
	}
	return false, nil
}

// FileAddedAt returns the author date of the commit that first introduced the
// file at path in the repository at repoRoot.  The path may be absolute; it is
// made relative to repoRoot before being passed to git so that --follow works
// correctly.  The second return value is false when the file has no recorded
// git history (untracked, or git unavailable).
func FileAddedAt(repoRoot, path string) (time.Time, bool) {
	addedAt, _, _, hasGit := FileAddedMeta(repoRoot, path)
	return addedAt, hasGit
}

// FileAddedMeta returns metadata for the commit that first introduced path,
// following renames to preserve original attribution.
func FileAddedMeta(repoRoot, path string) (time.Time, string, string, bool) {
	relPath, relErr := filepath.Rel(repoRoot, path)
	if relErr != nil {
		relPath = path
	}
	slog.Debug("resolving file add metadata", "path", relPath)

	// --diff-filter=A   : only commits where the file was *added*
	// --follow           : follow renames back to the original creation
	// Newest commit first; we want the last line (the original add).
	raw, err := RunGit(repoRoot,
		"log", "--diff-filter=A", "--follow", "--format=__CHAGG_ADD__%aI|%H", "--name-only", "--", relPath)
	if err != nil {
		return time.Time{}, "", "", false
	}

	lines := SplitLines(strings.TrimSpace(raw))
	if len(lines) == 0 {
		return time.Time{}, "", "", false
	}

	var currentTime time.Time
	var currentHash string
	oldestTime := time.Time{}
	oldestHash := ""
	oldestFile := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "__CHAGG_ADD__") {
			payload := strings.TrimPrefix(line, "__CHAGG_ADD__")
			parts := strings.SplitN(payload, "|", 2)
			if len(parts) != 2 {
				currentTime = time.Time{}
				currentHash = ""
				continue
			}

			t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
			if parseErr != nil {
				currentTime = time.Time{}
				currentHash = ""
				continue
			}

			currentTime = t
			currentHash = strings.TrimSpace(parts[1])
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		oldestTime = currentTime
		oldestHash = currentHash
		oldestFile = filepath.Base(strings.TrimSpace(filepath.FromSlash(line)))
	}

	if oldestTime.IsZero() {
		return time.Time{}, "", "", false
	}

	return oldestTime, oldestHash, oldestFile, true
}

// FileAddedMeta holds the metadata returned by FileAddedMetaMany for a single file.
type FileMetaResult struct {
	AddedAt          time.Time
	AddedCommitHash  string
	OriginalFilename string
}

// FileAddedMetaMany returns add metadata for multiple files using a single
// git log call. This avoids the O(N) subprocess overhead of calling
// FileAddedMeta per file. Paths may be absolute; the returned map key is
// the original path. Files missing from the result (e.g. untracked or
// renamed) should fall back to per-file FileAddedMeta for --follow semantics.
func FileAddedMetaMany(repoRoot string, paths []string) map[string]FileMetaResult {
	if len(paths) == 0 {
		return map[string]FileMetaResult{}
	}

	relToPath := make(map[string]string, len(paths))
	var relPaths []string
	for _, path := range paths {
		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			continue
		}
		cleanRel := filepath.Clean(relPath)
		relToPath[cleanRel] = path
		relPaths = append(relPaths, cleanRel)
	}

	if len(relPaths) == 0 {
		return map[string]FileMetaResult{}
	}

	// --diff-filter=A returns only commits where a file was added.
	// Without --follow: works for multiple paths in a single call.
	// Format: date|hash on the marker line, then filename lines below.
	args := []string{"log", "--diff-filter=A", "--format=__CHAGG_ADD__%aI|%H", "--name-only", "--"}
	args = append(args, relPaths...)

	raw, err := RunGit(repoRoot, args...)
	if err != nil {
		return map[string]FileMetaResult{}
	}

	result := make(map[string]FileMetaResult)
	var currentTime time.Time
	var currentHash string

	for _, line := range SplitLines(raw) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "__CHAGG_ADD__") {
			payload := strings.TrimPrefix(line, "__CHAGG_ADD__")
			parts := strings.SplitN(payload, "|", 2)
			if len(parts) != 2 {
				currentTime = time.Time{}
				currentHash = ""
				continue
			}
			t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
			if parseErr != nil {
				currentTime = time.Time{}
				currentHash = ""
				continue
			}
			currentTime = t
			currentHash = strings.TrimSpace(parts[1])
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		cleanRel := filepath.Clean(filepath.FromSlash(line))
		origPath, ok := relToPath[cleanRel]
		if !ok {
			continue
		}

		// Keep overwriting: git log walks newest→oldest, so the last
		// write for each file is the oldest add commit.
		result[origPath] = FileMetaResult{
			AddedAt:          currentTime,
			AddedCommitHash:  currentHash,
			OriginalFilename: filepath.Base(cleanRel),
		}
	}

	return result
}

// FindGitRoot walks up from startPath until it finds a directory that contains a
// ".git" entry. It returns the root path and true when found. If the filesystem
// root is reached without finding ".git", it returns startPath and false.
func FindGitRoot(startPath string) (string, bool, error) {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return "", false, err
	}
	slog.Debug("searching for git root", "startPath", current)

	for {
		gitPath := filepath.Join(current, ".git")
		if _, gitErr := os.Stat(gitPath); gitErr == nil {
			slog.Info("found git root", "path", current)
			return current, true, nil
		} else if !errors.Is(gitErr, os.ErrNotExist) {
			return "", false, gitErr
		}

		parent := filepath.Dir(current)
		if parent == current {
			slog.Info("no git root found, using CWD as boundary", "path", current)
			return current, false, nil
		}
		current = parent
	}
}

// FindAllChangesDirs recursively finds all ".changes" directories under root.
// It does not recurse into ".git" directories or into ".changes" directories
// themselves (nested ".changes" hierarchies are not supported).
// All returned paths are guaranteed to be within root.
func FindAllChangesDirs(root string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	slog.Debug("scanning for .changes dirs", "root", absRoot)

	var dirs []string

	walkErr := filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		if name == ".changes" {
			// Defensive: verify the found dir is within root (should always be true).
			if !IsWithinDir(absRoot, path) {
				slog.Warn("skipping .changes dir outside repo root", "path", path, "root", absRoot)
				return filepath.SkipDir
			}
			slog.Debug("found .changes dir", "path", path)
			dirs = append(dirs, path)
			return filepath.SkipDir
		}

		// Skip .git and other hidden directories (they will never contain .changes).
		if name != "." && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}

		return nil
	})

	slog.Info("found .changes directories", "count", len(dirs))
	return dirs, walkErr
}
