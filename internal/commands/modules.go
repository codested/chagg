package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

// ModuleInfo is the public representation of a resolved module, used for both
// text and JSON output.
type ModuleInfo struct {
	Name       string `json:"name"`
	ChangesDir string `json:"changes_dir"` // repo-root-relative, forward slashes
	TagPrefix  string `json:"tag_prefix"`
}

// ConfigModulesSubcommand returns the modules subcommand for use under config.
func ConfigModulesSubcommand() *cli.Command {
	return &cli.Command{
		Name:  "modules",
		Usage: "List all discovered modules and their .changes directories",
		Description: "Resolves all modules for the current repository and prints their " +
			"name, .changes directory (relative to the repo root), and tag prefix. " +
			"Use --format json to get machine-readable output suitable for scripting " +
			"and CI workflows (e.g. to discover which paths to diff in a GitHub Action).",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: text or json",
				Value: "text",
			},
		},
		Action: modulesAction,
	}
}

func modulesAction(_ context.Context, cmd *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	repoRoot, hasGit, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return err
	}
	if !hasGit {
		return changeentry.NewValidationError("git", "not inside a git repository")
	}

	infos, err := resolveModuleInfos(repoRoot)
	if err != nil {
		return err
	}

	switch cmd.String("format") {
	case "json":
		return renderModulesJSON(infos, os.Stdout)
	default:
		return renderModulesText(infos, os.Stdout)
	}
}

// resolveModuleInfos discovers all modules under repoRoot and returns their
// public representation with repo-root-relative paths.
func resolveModuleInfos(repoRoot string) ([]ModuleInfo, error) {
	changesDirs, err := gitutil.FindAllChangesDirs(repoRoot)
	if err != nil {
		return nil, err
	}

	if len(changesDirs) == 0 {
		return []ModuleInfo{}, nil
	}

	moduleMap, err := changeentry.ResolveModulesForChangesDirs(repoRoot, changesDirs)
	if err != nil {
		return nil, err
	}

	infos := make([]ModuleInfo, 0, len(changesDirs))
	for _, dir := range changesDirs {
		m := moduleMap[dir]
		relDir, relErr := filepath.Rel(repoRoot, m.ChangesDir)
		if relErr != nil {
			relDir = m.ChangesDir
		}
		infos = append(infos, ModuleInfo{
			Name:       m.Name,
			ChangesDir: filepath.ToSlash(relDir),
			TagPrefix:  m.TagPrefix,
		})
	}
	return infos, nil
}

func renderModulesText(infos []ModuleInfo, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "NAME\tCHANGES-DIR\tTAG-PREFIX\n")
	for _, m := range infos {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Name, m.ChangesDir, m.TagPrefix)
	}
	return tw.Flush()
}

func renderModulesJSON(infos []ModuleInfo, w io.Writer) error {
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal modules: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
