package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/urfave/cli/v3"
)

func ReleaseCommand() *cli.Command {
	return &cli.Command{
		Name:    "rel",
		Aliases: []string{"r", "release"},
		Usage:   "Create the next release tag from staging changes",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "pre",
				Usage: "Optional pre-release channel, e.g. beta, staging, preprod",
			},
			&cli.StringFlag{
				Name:  "build",
				Usage: "Optional build metadata, e.g. build.42",
			},
		},
		Action: releaseAction,
	}
}

var semverIdentifierPattern = regexp.MustCompile(`^[0-9A-Za-z.-]+$`)

const (
	bumpPatch = iota
	bumpMinor
	bumpMajor
)

func releaseAction(_ context.Context, _ *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, _, err := changeentry.FindGitRoot(cwd)
	if err != nil {
		return err
	}

	if err := ensureCleanWorkingTree(repoRoot); err != nil {
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

	cl, err := changelog.LoadChangeLog(repoRoot, module, changelog.FilterOptions{})
	if err != nil {
		return err
	}

	staging := changelog.StagingOnly(cl)
	if len(staging.Groups) == 0 || staging.Groups[0].TotalEntries() == 0 {
		fmt.Println("No staging changes found. Nothing to release.")
		return nil
	}

	tags, err := changelog.ListSemVerTags(repoRoot, module.TagPrefix)
	if err != nil {
		return err
	}

	preReleaseLabel := strings.TrimSpace(cmd.String("pre"))
	if preReleaseLabel != "" && !semverIdentifierPattern.MatchString(preReleaseLabel) {
		return changeentry.NewValidationError("pre", "--pre must contain only [0-9A-Za-z.-]")
	}

	buildMetadata := strings.TrimSpace(cmd.String("build"))
	if buildMetadata != "" && !semverIdentifierPattern.MatchString(buildMetadata) {
		return changeentry.NewValidationError("build", "--build must contain only [0-9A-Za-z.-]")
	}

	latestStableTag, hasStableTag := latestStableTag(tags)

	var nextVersion changelog.SemVersion
	withVPrefix := true

	if !hasStableTag {
		enteredVersion, promptErr := promptFirstVersion(os.Stdin, os.Stdout, isInteractiveStdin())
		if promptErr != nil {
			return promptErr
		}

		parsedVersion, hasVPrefix, parseErr := changelog.ParseSemVersion(enteredVersion)
		if parseErr != nil {
			return changeentry.NewValidationError("version", fmt.Sprintf("invalid SemVer version: %q", enteredVersion))
		}
		nextVersion = parsedVersion
		withVPrefix = hasVPrefix
	} else {
		withVPrefix = latestStableTag.HasVPrefix
		bump := detectBumpLevel(staging.Groups[0])
		nextVersion = bumpVersion(latestStableTag.Version, bump)
	}

	if preReleaseLabel != "" {
		nextVersion = changelog.NextPreReleaseLabelVersion(nextVersion, preReleaseLabel, tags)
	}

	if buildMetadata != "" {
		nextVersion.Build = buildMetadata
	}

	versionText := nextVersion.String(withVPrefix)
	tagName := module.TagPrefix + versionText

	if err = createLocalTag(repoRoot, tagName); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return changeentry.NewConflictError(fmt.Sprintf("tag already exists: %s", tagName))
		}
		return err
	}

	entryCount := staging.Groups[0].TotalEntries()
	fmt.Printf("Created local tag %s for module %q from %d staging %s.\n", tagName, module.Name, entryCount, pluralize(entryCount, "entry", "entries"))
	fmt.Println("Tag was created locally and was not pushed.")
	fmt.Printf("To push it, run:\n\n  git push origin %s\n", tagName)

	return nil
}

func detectBumpLevel(group changelog.VersionGroup) int {
	bump := bumpPatch
	for _, tg := range group.TypeGroups {
		for _, entry := range tg.Entries {
			if entry.Entry.Breaking || entry.Entry.Type == changeentry.ChangeTypeRemoval {
				return bumpMajor
			}
			if entry.Entry.Type == changeentry.ChangeTypeFeature {
				bump = bumpMinor
			}
		}
	}

	return bump
}

func bumpVersion(version changelog.SemVersion, level int) changelog.SemVersion {
	next := changelog.SemVersion{Major: version.Major, Minor: version.Minor, Patch: version.Patch}

	switch level {
	case bumpMajor:
		next.Major++
		next.Minor = 0
		next.Patch = 0
	case bumpMinor:
		next.Minor++
		next.Patch = 0
	default:
		next.Patch++
	}

	return next
}

func promptFirstVersion(input *os.File, output *os.File, interactive bool) (string, error) {
	const defaultVersion = "0.1.0"
	if !interactive {
		fmt.Fprintf(output, "No stable SemVer tag found. Using default initial release version %s (non-interactive).\n", defaultVersion)
		return defaultVersion, nil
	}

	reader := bufio.NewReader(input)
	fmt.Fprintf(output, "No stable SemVer tag found. Enter initial release version [%s]: ", defaultVersion)
	line, err := reader.ReadString('\n')
	if err != nil {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return defaultVersion, nil
		}
		return trimmed, nil
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return defaultVersion, nil
	}

	return trimmed, nil
}

func createLocalTag(repoRoot string, version string) error {
	cmd := exec.Command("git", "tag", version)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("create git tag %s: %s", version, msg)
	}

	return nil
}

func ensureCleanWorkingTree(repoRoot string) error {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("check git working tree: %s", msg)
	}

	if strings.TrimSpace(string(out)) != "" {
		return changeentry.NewConflictError("release aborted: uncommitted changes detected; commit, stash, or discard changes before running chagg release")
	}

	return nil
}

func pluralize(n int, singular string, plural string) string {
	if n == 1 {
		return singular
	}

	return plural
}

func latestStableTag(tags []changelog.Tag) (changelog.Tag, bool) {
	for i := len(tags) - 1; i >= 0; i-- {
		if !tags[i].Version.IsPreRelease() {
			return tags[i], true
		}
	}

	return changelog.Tag{}, false
}
