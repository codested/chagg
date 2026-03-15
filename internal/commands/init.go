package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/codested/chagg/internal/semver"
	"github.com/urfave/cli/v3"
)

// InitCommand returns the init command. cio abstracts filesystem I/O so that
// tests can inject a MockConfigIO.
func InitCommand(cio changeentry.ConfigIO) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize .changes directories and module configuration",
		Description: "Set up chagg for a repository. " +
			"Run from the repo root or a sub-directory. " +
			"In interactive mode you will be guided through single-module or " +
			"multi-module setup. Use --no-prompt for non-interactive defaults.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "no-prompt",
				Usage: "Use defaults for all prompts (non-interactive mode)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			noPrompt := cmd.Bool("no-prompt") || !isInteractiveStdin()
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}
			return runInit(cwd, cio, os.Stdin, os.Stdout, noPrompt)
		},
	}
}

// runInit is the testable core of the init command. cwd is the current working
// directory; all other I/O goes through the supplied reader/writer and ConfigIO.
func runInit(cwd string, cio changeentry.ConfigIO, in io.Reader, out io.Writer, noPrompt bool) error {
	repoRoot, hasGitRoot, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return err
	}
	if !hasGitRoot {
		return changeentry.NewValidationError("git", "not inside a git repository; run 'git init' first")
	}

	p := newInitPrompter(in, out, noPrompt)

	if !isAtRepoRoot(cwd, repoRoot) {
		return initFromSubdir(p, cio, cwd, repoRoot)
	}
	return initFromRepoRoot(p, cio, repoRoot)
}

// ── prompter ──────────────────────────────────────────────────────────────────

type initPrompter struct {
	reader   *bufio.Reader
	writer   io.Writer
	noPrompt bool
}

func newInitPrompter(in io.Reader, out io.Writer, noPrompt bool) *initPrompter {
	return &initPrompter{reader: bufio.NewReader(in), writer: out, noPrompt: noPrompt}
}

func (p *initPrompter) printf(format string, args ...any) {
	fmt.Fprintf(p.writer, format, args...)
}

func (p *initPrompter) println(s string) {
	fmt.Fprintln(p.writer, s)
}

// ask prompts for a text answer. When noPrompt is true the default is returned
// immediately (and printed so the user can see which value was chosen).
func (p *initPrompter) ask(question, defaultAnswer string) string {
	if p.noPrompt {
		if defaultAnswer != "" {
			p.printf("%s → %s\n", question, defaultAnswer)
		}
		return defaultAnswer
	}
	if defaultAnswer != "" {
		p.printf("%s [%s]: ", question, defaultAnswer)
	} else {
		p.printf("%s: ", question)
	}
	line, _ := p.reader.ReadString('\n')
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return defaultAnswer
	}
	return trimmed
}

// yesNo prompts for a yes/no answer. defaultYes controls the default when the
// user presses Enter without typing, or when noPrompt is true.
func (p *initPrompter) yesNo(question string, defaultYes bool) bool {
	opts := "y/N"
	if defaultYes {
		opts = "Y/n"
	}
	if p.noPrompt {
		answer := "no"
		if defaultYes {
			answer = "yes"
		}
		p.printf("%s [%s] → %s\n", question, opts, answer)
		return defaultYes
	}
	p.printf("%s [%s]: ", question, opts)
	line, _ := p.reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return defaultYes
	}
}

// ── flows ─────────────────────────────────────────────────────────────────────

func isAtRepoRoot(cwd, repoRoot string) bool {
	return filepath.Clean(cwd) == filepath.Clean(repoRoot)
}

// initFromSubdir handles the case where the user is inside a sub-directory of
// the repo. It prompts whether to create a module for the current directory or
// fall back to a full repo-root initialization.
func initFromSubdir(p *initPrompter, cio changeentry.ConfigIO, cwd, repoRoot string) error {
	relCWD, err := filepath.Rel(repoRoot, cwd)
	if err != nil {
		relCWD = cwd
	}

	p.printf("Current directory (%s) is not the repo root.\n", relCWD)

	initHere := p.yesNo(fmt.Sprintf("Initialize .changes here as a module for %q?", relCWD), true)
	if !initHere {
		p.println("Falling back to repo-root initialization.")
		return initFromRepoRoot(p, cio, repoRoot)
	}

	defaultName := filepath.Base(cwd)
	name := p.ask("Module name", defaultName)

	changesRelDir := filepath.ToSlash(filepath.Join(relCWD, ".changes"))
	changesRelDir = p.ask("Changes directory (relative to repo root)", changesRelDir)

	defaultTagPrefix := name + "-"
	tagPrefix := p.ask("Tag prefix (empty for none)", defaultTagPrefix)

	return writeModuleConfig(p, cio, repoRoot, changeentry.RawModule{
		Name:       name,
		ChangesDir: changesRelDir,
		TagPrefix:  tagPrefix,
	})
}

// initFromRepoRoot handles initialization when the user is at the repo root.
// It asks whether this is a multi-module repo, then delegates accordingly.
func initFromRepoRoot(p *initPrompter, cio changeentry.ConfigIO, repoRoot string) error {
	multiModule := p.yesNo("Is this a multi-module project?", false)
	if !multiModule {
		return initSingleModule(p, repoRoot)
	}
	return initMultiModule(p, cio, repoRoot)
}

// initSingleModule creates a single .changes directory at the repo root without
// writing any config file (the defaults are fine for a simple repo).
func initSingleModule(p *initPrompter, repoRoot string) error {
	changesDir := filepath.Join(repoRoot, ".changes")
	if err := createChangesDir(p, changesDir); err != nil {
		return err
	}
	printNextSteps(p, false)
	return nil
}

// initMultiModule prompts for one or more module definitions, checks for
// potentially conflicting existing plain-version tags, then creates .changes
// directories and writes a .chagg.yaml with the modules list.
func initMultiModule(p *initPrompter, cio changeentry.ConfigIO, repoRoot string) error {
	if err := warnAboutPlainTags(p, repoRoot); err != nil {
		// Tag listing failures are non-fatal — just skip the warning.
		_ = err
	}

	modules, err := collectModules(p, repoRoot)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		p.println("No modules provided. Nothing to do.")
		return nil
	}

	for _, m := range modules {
		dir := filepath.Join(repoRoot, filepath.FromSlash(m.ChangesDir))
		if err := createChangesDir(p, dir); err != nil {
			return err
		}
	}

	cfg, _, err := cio.ReadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &changeentry.RawConfig{}
	}
	cfg.Modules = modules

	name, err := cio.WriteRepoConfig(repoRoot, cfg)
	if err != nil {
		return err
	}
	p.printf("Wrote %s\n", name)
	printNextSteps(p, true)
	return nil
}

// collectModules interactively collects one or more module definitions.
func collectModules(p *initPrompter, repoRoot string) ([]changeentry.RawModule, error) {
	var modules []changeentry.RawModule

	p.println("\nDefine modules (enter an empty name when you are done):")
	for {
		p.printf("\nModule %d:\n", len(modules)+1)

		name := p.ask("  Name", "")
		if name == "" {
			if len(modules) == 0 {
				p.println("  At least one module name is required.")
				// In non-interactive mode we cannot loop; bail out.
				if p.noPrompt {
					return nil, nil
				}
				continue
			}
			break
		}

		defaultDir := filepath.ToSlash(filepath.Join(name, ".changes"))
		changesDir := p.ask("  Changes directory (relative to repo root)", defaultDir)

		defaultPrefix := name + "-"
		tagPrefix := p.ask("  Tag prefix (empty for none)", defaultPrefix)

		modules = append(modules, changeentry.RawModule{
			Name:       name,
			ChangesDir: changesDir,
			TagPrefix:  tagPrefix,
		})

		if !p.yesNo("Add another module?", false) {
			break
		}
	}
	return modules, nil
}

// writeModuleConfig appends (or replaces) a single module entry in the repo
// config and writes it back.
func writeModuleConfig(p *initPrompter, cio changeentry.ConfigIO, repoRoot string, module changeentry.RawModule) error {
	dir := filepath.Join(repoRoot, filepath.FromSlash(module.ChangesDir))
	if err := createChangesDir(p, dir); err != nil {
		return err
	}

	cfg, _, err := cio.ReadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &changeentry.RawConfig{}
	}

	replaced := false
	for i, m := range cfg.Modules {
		if strings.EqualFold(m.Name, module.Name) {
			cfg.Modules[i] = module
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Modules = append(cfg.Modules, module)
	}

	name, err := cio.WriteRepoConfig(repoRoot, cfg)
	if err != nil {
		return err
	}
	p.printf("Wrote %s\n", name)
	printNextSteps(p, true)
	return nil
}

// warnAboutPlainTags lists existing plain SemVer tags (those without a module
// prefix) and warns the user they may conflict with future module releases.
// If the user declines to continue, an error is returned.
func warnAboutPlainTags(p *initPrompter, repoRoot string) error {
	tags, err := gitutil.ListSemVerTags(repoRoot, "")
	if err != nil || len(tags) == 0 {
		return err
	}

	// Filter to tags that are exactly a bare SemVer (no module prefix): they
	// are already returned by ListSemVerTags with an empty prefix filter.
	plainTags := make([]semver.Tag, 0, len(tags))
	for _, t := range tags {
		stripped := t.Name
		if strings.HasPrefix(stripped, "v") {
			stripped = stripped[1:]
		}
		if _, _, parseErr := semver.ParseSemVersion(stripped); parseErr == nil {
			plainTags = append(plainTags, t)
		}
	}
	if len(plainTags) == 0 {
		return nil
	}

	p.printf("\nWarning: %d existing version tag(s) without a module prefix found:\n", len(plainTags))
	for _, t := range plainTags {
		p.printf("  %s\n", t.Name)
	}
	p.println("These tags may be interpreted as releases belonging to no specific module,")
	p.println("which can cause unexpected version assignments in a multi-module setup.")

	if !p.yesNo("Continue with multi-module setup anyway?", true) {
		return changeentry.NewValidationError("init", "aborted by user")
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func createChangesDir(p *initPrompter, dir string) error {
	if _, err := os.Stat(dir); err == nil {
		p.printf("%s already exists, skipping.\n", dir)
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	p.printf("Created %s\n", dir)
	return nil
}

func printNextSteps(p *initPrompter, multiModule bool) {
	p.println("")
	p.println("Done. You may also want to configure:")
	p.println("  chagg config defaults.audience <value>        # default audience for new entries")
	p.println("  chagg config git.write.push-release-tag true  # push tags to origin automatically")
	if multiModule {
		p.println("  chagg config types                            # view available change types per module")
	} else {
		p.println("  chagg config types                            # view available change types")
	}
	p.println("")
	p.println("To add your first change entry, run:")
	p.println("  chagg add <path>")
}
