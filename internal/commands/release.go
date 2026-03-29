package commands

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/codested/chagg/internal/semver"
	"github.com/urfave/cli/v3"
)

func ReleaseCommand() *cli.Command {
	return &cli.Command{
		Name:      "release",
		Aliases:   []string{"r", "rel"},
		Usage:     "Create the next release tag from staging changes",
		ArgsUsage: "[version]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "Compute the next version without creating a tag",
			},
			&cli.BoolFlag{
				Name:  "version-only",
				Usage: "Print only the computed version and exit",
			},
			&cli.BoolFlag{
				Name:  "push",
				Usage: "Push the created tag to origin",
			},
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

func releaseAction(_ context.Context, cmd *cli.Command) error {
	mode, err := resolveReleaseMode(cmd)
	if err != nil {
		return err
	}
	quiet := cmd.Root().Bool("quiet")

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

	// Apply config-driven auto-push when --push was not explicitly given.
	// push-release-tag = true in any config layer triggers an automatic push
	// after the tag is created, without requiring the --push flag.
	applyConfigPushOverride(cmd, &mode, module.GitWrite)

	if mode.willCreateTag && !module.GitWrite.AllowsReleaseTag() {
		return changeentry.NewValidationError("config", "release tag creation is disabled by git-write policy")
	}
	// Push is gated only by the global kill-switch. push-release-tag controls
	// automatic push behaviour, not whether --push is permitted.
	if mode.pushTag && !module.GitWrite.Enabled {
		return changeentry.NewValidationError("config", "git write operations are disabled by git-write policy")
	}

	if mode.requiresGitWrites() {
		if err := ensureCleanWorkingTree(repoRoot); err != nil {
			return err
		}
		if err := ensureNoPendingPush(repoRoot); err != nil {
			return err
		}
	}

	cl, err := changelog.LoadChangeLog(repoRoot, module, changelog.FilterOptions{})
	if err != nil {
		return err
	}

	staging := changelog.StagingOnly(cl)
	if len(staging.Groups) == 0 || staging.Groups[0].TotalEntries() == 0 {
		if !quiet {
			fmt.Println("No staging changes found. Nothing to release.")
		}
		return nil
	}

	tags, err := gitutil.ListSemVerTags(repoRoot, module.TagPrefix)
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

	latestTag, hasStableTag := semver.LatestStable(tags)

	var nextVersion semver.SemVersion
	withVPrefix := true

	explicitVersion := strings.TrimSpace(cmd.Args().Get(0))

	if explicitVersion != "" {
		var parseErr error
		nextVersion, withVPrefix, parseErr = parseExplicitVersion(explicitVersion)
		if parseErr != nil {
			return parseErr
		}
	} else if !hasStableTag {
		enteredVersion, promptErr := promptFirstVersion(os.Stdin, os.Stdout, isInteractiveStdin())
		if promptErr != nil {
			return promptErr
		}

		parsedVersion, hasVP, parseErr := semver.ParseSemVersion(enteredVersion)
		if parseErr != nil {
			return changeentry.NewValidationError("version", fmt.Sprintf("invalid SemVer version: %q", enteredVersion))
		}
		nextVersion = parsedVersion
		withVPrefix = hasVP
	} else {
		withVPrefix = latestTag.HasVPrefix
		bump := detectBumpLevel(staging.Groups[0], module.Types)
		nextVersion = semver.Bump(latestTag.Version, bump)
	}

	// Apply release.v-prefix policy override.
	switch module.Release.VPrefix {
	case "always":
		withVPrefix = true
	case "never":
		withVPrefix = false
		// "auto": keep the value already determined from tag history / user input
	}

	if preReleaseLabel != "" {
		nextVersion = semver.NextPreReleaseLabelVersion(nextVersion, preReleaseLabel, tags)
	}

	if buildMetadata != "" {
		nextVersion.Build = buildMetadata
	}

	versionText := nextVersion.String(withVPrefix)
	tagName := module.TagPrefix + versionText

	if mode.versionOnly {
		fmt.Println(versionText)
		return nil
	}

	// Determine alias tags (before dryRun block so dry-run can report them too).
	createAliases, aliasErr := resolveAliasTags(repoRoot, module)
	if aliasErr != nil {
		return aliasErr
	}
	var majorAlias, minorAlias string
	if createAliases && nextVersion.PreRelease == "" {
		vStr := ""
		if withVPrefix {
			vStr = "v"
		}
		majorAlias = fmt.Sprintf("%s%s%d", module.TagPrefix, vStr, nextVersion.Major)
		minorAlias = fmt.Sprintf("%s%s%d.%d", module.TagPrefix, vStr, nextVersion.Major, nextVersion.Minor)
	}

	entryCount := staging.Groups[0].TotalEntries()
	if mode.dryRun {
		if !quiet {
			fmt.Printf("Dry-run: would create local tag %s%s from %d staging %s.\n", tagName, moduleClause(module.Name), entryCount, pluralize(entryCount, "entry", "entries"))
			if majorAlias != "" {
				fmt.Printf("Dry-run: would create/update alias tags %s and %s.\n", majorAlias, minorAlias)
			}
			if module.GitWrite.ReleasePush {
				fmt.Printf("Dry-run: would push tag with: git push origin %s (auto-push from config)\n", tagName)
			}
		}
		return nil
	}

	if err = createLocalTag(repoRoot, tagName); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return changeentry.NewConflictError(fmt.Sprintf("tag already exists: %s", tagName))
		}
		return err
	}

	// Create alias tags locally.
	if majorAlias != "" {
		if err := createOrUpdateLocalTag(repoRoot, majorAlias); err != nil {
			return err
		}
		if err := createOrUpdateLocalTag(repoRoot, minorAlias); err != nil {
			return err
		}
	}

	if !quiet {
		fmt.Printf("Created local tag %s%s from %d staging %s.\n", tagName, moduleClause(module.Name), entryCount, pluralize(entryCount, "entry", "entries"))
		if majorAlias != "" {
			fmt.Printf("Created/updated alias tags %s and %s.\n", majorAlias, minorAlias)
		}
	}

	if mode.pushTag {
		if err := pushTag(repoRoot, tagName); err != nil {
			return err
		}
		if majorAlias != "" {
			if err := pushTagForce(repoRoot, majorAlias); err != nil {
				return err
			}
			if err := pushTagForce(repoRoot, minorAlias); err != nil {
				return err
			}
			if !quiet {
				fmt.Printf("Pushed alias tags %s and %s to origin.\n", majorAlias, minorAlias)
			}
		}
		if !quiet {
			fmt.Printf("Pushed tag %s to origin.\n", tagName)
		}
		return nil
	}

	if !quiet {
		fmt.Println("Tag was created locally and was not pushed.")
		fmt.Printf("To push it, run:\n\n  git push origin %s\n", tagName)
	}

	return nil
}

type releaseMode struct {
	dryRun        bool
	versionOnly   bool
	pushTag       bool
	willCreateTag bool
}

func resolveReleaseMode(cmd *cli.Command) (releaseMode, error) {
	mode := releaseMode{
		dryRun:      cmd.Bool("dry-run"),
		versionOnly: cmd.Bool("version-only"),
		pushTag:     cmd.Bool("push"),
	}

	if mode.versionOnly && mode.pushTag {
		return releaseMode{}, changeentry.NewValidationError("flags", "--version-only cannot be combined with --push")
	}
	if mode.versionOnly && mode.dryRun {
		return releaseMode{}, changeentry.NewValidationError("flags", "--version-only cannot be combined with --dry-run")
	}
	if mode.dryRun && mode.pushTag {
		return releaseMode{}, changeentry.NewValidationError("flags", "--dry-run cannot be combined with --push")
	}

	mode.willCreateTag = !mode.dryRun && !mode.versionOnly
	return mode, nil
}

// applyConfigPushOverride sets mode.pushTag when the git-write policy has
// ReleasePush enabled and the user did not explicitly pass --push. It is a
// no-op in dry-run and version-only modes (where willCreateTag is already
// false) and when the caller has already set --push explicitly.
func applyConfigPushOverride(cmd *cli.Command, mode *releaseMode, policy changeentry.GitWritePolicy) {
	if !cmd.IsSet("push") && mode.willCreateTag && policy.ReleasePush {
		mode.pushTag = true
	}
}

func (m releaseMode) requiresGitWrites() bool {
	return m.willCreateTag || m.pushTag
}

func detectBumpLevel(group changelog.VersionGroup, registry changeentry.TypeRegistry) int {
	level := semver.BumpPatch
	for _, tg := range group.TypeGroups {
		for _, entry := range tg.Entries {
			entryLevel := effectiveBumpInt(entry.Entry, registry)
			if entryLevel > level {
				level = entryLevel
				if level == semver.BumpMajor {
					return semver.BumpMajor
				}
			}
		}
	}
	return level
}

// effectiveBumpInt returns the resolved integer bump level for a single entry.
// It respects an explicit Bump override, falling back to the type-based default
// from the provided registry.
func effectiveBumpInt(entry changeentry.Entry, registry changeentry.TypeRegistry) int {
	bump := entry.Bump
	if bump == "" {
		bump = registry.DefaultBumpLevel(entry.Type)
	}
	switch bump {
	case changeentry.BumpLevelMajor:
		return semver.BumpMajor
	case changeentry.BumpLevelMinor:
		return semver.BumpMinor
	default:
		return semver.BumpPatch
	}
}

func promptFirstVersion(input *os.File, output *os.File, interactive bool) (string, error) {
	const defaultVersion = "0.1.0"
	if !interactive {
		_, _ = fmt.Fprintf(output, "No stable SemVer tag found. Using default initial release version %s (non-interactive).\n", defaultVersion)
		return defaultVersion, nil
	}

	reader := bufio.NewReader(input)
	_, _ = fmt.Fprintf(output, "No stable SemVer tag found. Enter initial release version [%s]: ", defaultVersion)
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
	if _, err := gitutil.RunGit(repoRoot, "tag", version); err != nil {
		return fmt.Errorf("create git tag %s: %w", version, err)
	}
	return nil
}

func pushTag(repoRoot string, version string) error {
	if _, err := gitutil.RunGit(repoRoot, "push", "origin", version); err != nil {
		return fmt.Errorf("push git tag %s: %w", version, err)
	}
	return nil
}

// resolveAliasTags returns true when alias (major and major.minor) tags
// should be created alongside the full semver release tag.
func resolveAliasTags(repoRoot string, module changeentry.ModuleConfig) (bool, error) {
	switch module.Release.AliasTags {
	case "always":
		return true, nil
	case "never":
		return false, nil
	default: // "auto"
		return gitutil.HasAbbreviatedVersionTags(repoRoot, module.TagPrefix)
	}
}

// createOrUpdateLocalTag creates a git tag, force-updating it if it already exists.
// Alias tags are expected to move with each release.
func createOrUpdateLocalTag(repoRoot, tagName string) error {
	if _, err := gitutil.RunGit(repoRoot, "tag", "-f", tagName); err != nil {
		return fmt.Errorf("create/update git tag %s: %w", tagName, err)
	}
	return nil
}

// pushTagForce force-pushes a tag to origin, needed for alias tags that move
// with each release.
func pushTagForce(repoRoot, tagName string) error {
	if _, err := gitutil.RunGit(repoRoot, "push", "origin", "-f", tagName); err != nil {
		return fmt.Errorf("push git tag %s: %w", tagName, err)
	}
	return nil
}

// ensureNoPendingPush returns an error when the current branch has local
// commits that have not been pushed to its upstream tracking branch. A tag
// created on an unpushed commit would be unreachable on the remote after
// pushing the tag alone.
//
// The check is silently skipped when there is no upstream configured (e.g.
// a local-only branch or a detached HEAD), so it never blocks offline
// workflows.
func ensureNoPendingPush(repoRoot string) error {
	out, err := gitutil.RunGit(repoRoot, "rev-list", "--count", "@{u}..HEAD")
	if err != nil {
		// No upstream tracking branch — nothing to enforce.
		slog.Debug("skipping unpushed-commits check: no upstream branch")
		return nil
	}

	n := strings.TrimSpace(out)
	if n == "0" || n == "" {
		return nil
	}

	return changeentry.NewValidationError("release",
		fmt.Sprintf("current branch has %s unpushed commit(s); push to the upstream branch before releasing so the tag points to a commit the remote already has", n))
}

func ensureCleanWorkingTree(repoRoot string) error {
	out, err := gitutil.RunGit(repoRoot, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("check git working tree: %w", err)
	}

	if strings.TrimSpace(out) != "" {
		return changeentry.NewConflictError("release aborted: uncommitted changes detected; commit, stash, or discard changes before running chagg release")
	}

	return nil
}

// parseExplicitVersion parses a user-supplied version string and returns
// only the base version (major.minor.patch). Pre-release and build metadata
// from the input are intentionally stripped because they are controlled by
// the --pre and --build flags.
func parseExplicitVersion(raw string) (semver.SemVersion, bool, error) {
	parsed, hasVPrefix, err := semver.ParseSemVersion(raw)
	if err != nil {
		return semver.SemVersion{}, false, changeentry.NewValidationError(
			"version", fmt.Sprintf("invalid SemVer version: %q", raw),
		)
	}
	return semver.SemVersion{
		Major: parsed.Major,
		Minor: parsed.Minor,
		Patch: parsed.Patch,
	}, hasVPrefix, nil
}

func pluralize(n int, singular string, plural string) string {
	if n == 1 {
		return singular
	}

	return plural
}
