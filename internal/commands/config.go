package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/urfave/cli/v3"
)

// ConfigCommand returns the config command. cio abstracts filesystem I/O so
// that tests can inject a MockConfigIO.
func ConfigCommand(cio changeentry.ConfigIO) *cli.Command {
	return &cli.Command{
		Name:    "config",
		Aliases: []string{"cfg"},
		Usage:   "Read and write chagg configuration",
		Description: "Inspect or modify chagg settings. " +
			"Without arguments, lists all resolved settings for the current module. " +
			"Pass a key to read it, or a key and value to write it. " +
			"Use --global to target the user config (~/.config/chagg/config.yaml); " +
			"the default scope is the repo config (.chagg.yaml).",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "list",
				Aliases: []string{"l"},
				Usage:   "List all resolved settings",
			},
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format for --list: text, json",
				Value: "text",
			},
			&cli.BoolFlag{
				Name:  "global",
				Usage: "Operate on the user-level config file",
			},
			&cli.BoolFlag{
				Name:  "unset",
				Usage: "Remove a key from the target config file",
			},
		},
		Commands: []*cli.Command{
			configTypesSubcommand(cio),
			ConfigModulesSubcommand(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return configAction(ctx, cmd, cio)
		},
	}
}

// ── subcommand: types ─────────────────────────────────────────────────────────

func configTypesSubcommand(cio changeentry.ConfigIO) *cli.Command {
	return &cli.Command{
		Name:  "types",
		Usage: "List all available change types for the current module",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "format",
				Usage: "Output format: text, json",
				Value: "text",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return configTypesAction(ctx, cmd, cio)
		},
	}
}

func configTypesAction(_ context.Context, cmd *cli.Command, cio changeentry.ConfigIO) error {
	module, err := resolveModuleOrDefault(cio)
	if err != nil {
		return err
	}

	format := strings.ToLower(strings.TrimSpace(cmd.String("format")))
	switch format {
	case "json":
		return renderTypesJSON(module.Types.Definitions(), os.Stdout)
	default:
		return renderTypes(module.Types.Definitions(), os.Stdout)
	}
}

func renderTypes(defs []changeentry.TypeDefinition, w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %-14s\t%-22s\t%-7s\t%-6s\t%s\n", "ID", "ALIASES", "BUMP", "ORDER", "TITLE")
	fmt.Fprintf(tw, "  %-14s\t%-22s\t%-7s\t%-6s\t%s\n",
		strings.Repeat("─", 14), strings.Repeat("─", 22),
		strings.Repeat("─", 7), strings.Repeat("─", 6), strings.Repeat("─", 20))
	for _, d := range defs {
		aliases := "—"
		if len(d.Aliases) > 0 {
			aliases = strings.Join(d.Aliases, ", ")
		}
		fmt.Fprintf(tw, "  %-14s\t%-22s\t%-7s\t%-6d\t%s\n",
			string(d.ID), aliases, string(d.DefaultBump), d.Order, d.Title)
	}
	return tw.Flush()
}

func renderTypesJSON(defs []changeentry.TypeDefinition, w io.Writer) error {
	type jsonType struct {
		ID          string   `json:"id"`
		Aliases     []string `json:"aliases,omitempty"`
		DefaultBump string   `json:"default_bump"`
		Order       int      `json:"order"`
		Title       string   `json:"title"`
	}

	types := make([]jsonType, 0, len(defs))
	for _, d := range defs {
		types = append(types, jsonType{
			ID:          string(d.ID),
			Aliases:     d.Aliases,
			DefaultBump: string(d.DefaultBump),
			Order:       d.Order,
			Title:       d.Title,
		})
	}

	doc := struct {
		SchemaVersion int        `json:"schema_version"`
		Types         []jsonType `json:"types"`
	}{SchemaVersion: 1, Types: types}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// ── main action ───────────────────────────────────────────────────────────────

func configAction(_ context.Context, cmd *cli.Command, cio changeentry.ConfigIO) error {
	args := cmd.Args().Slice()
	global := cmd.Bool("global")
	unset := cmd.Bool("unset")

	// No args or --list: show all resolved settings.
	if len(args) == 0 || cmd.Bool("list") {
		format := strings.ToLower(strings.TrimSpace(cmd.String("format")))
		if format == "json" {
			return configListJSON(cio, os.Stdout)
		}
		return configList(cio, os.Stdout)
	}

	key := args[0]
	quiet := cmd.Root().Bool("quiet")

	if unset {
		return configUnset(key, global, cio, quiet)
	}

	if len(args) == 1 {
		// Get resolved value.
		return configGet(key, cio, os.Stdout)
	}

	// Set value (remaining args join to support list values).
	values := args[1:]
	return configSet(key, values, global, cio, quiet)
}

// ── list ──────────────────────────────────────────────────────────────────────

func configList(cio changeentry.ConfigIO, w io.Writer) error {
	module, err := resolveModuleOrDefault(cio)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "Defaults:")
	fmt.Fprintf(w, "  defaults.audience             = %s\n", formatStringList(module.Defaults.Audience))
	fmt.Fprintf(w, "  defaults.rank                 = %d\n", module.Defaults.Rank)
	fmt.Fprintf(w, "  defaults.component            = %s\n", formatStringList(module.Defaults.Component))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Git write policy:")
	fmt.Fprintf(w, "  git.write.allow               = %t\n", module.GitWrite.Enabled)
	fmt.Fprintf(w, "  git.write.add-change          = %t\n", module.GitWrite.Add)
	fmt.Fprintf(w, "  git.write.create-release-tag  = %t\n", module.GitWrite.ReleaseTag)
	fmt.Fprintf(w, "  git.write.push-release-tag    = %t\n", module.GitWrite.ReleasePush)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Release policy:")
	fmt.Fprintf(w, "  release.v-prefix              = %s\n", module.Release.VPrefix)
	fmt.Fprintf(w, "  release.alias-tags            = %s\n", module.Release.AliasTags)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Types (use 'chagg config types' for details):")
	for _, d := range module.Types.Definitions() {
		fmt.Fprintf(w, "  %s\n", string(d.ID))
	}

	// Modules section: only shown when more than one module exists (multi-module
	// repo) or the repo has a root .changes directory.
	cwd, err := os.Getwd()
	if err != nil {
		return nil // non-fatal; skip modules listing
	}
	repoRoot, _, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return nil // non-fatal; skip modules listing
	}
	dirs, err := gitutil.FindAllChangesDirs(repoRoot)
	if err != nil || len(dirs) == 0 {
		return nil
	}
	moduleMap, err := changeentry.ResolveModulesForChangesDirs(repoRoot, dirs)
	if err != nil {
		return nil // non-fatal; skip modules listing
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Modules (use 'chagg config modules' for details):")
	for _, dir := range dirs {
		m := moduleMap[dir]
		label := m.Name
		if label == "" {
			label = "(root)"
		}
		fmt.Fprintf(w, "  %s\n", label)
	}
	return nil
}

func configListJSON(cio changeentry.ConfigIO, w io.Writer) error {
	module, err := resolveModuleOrDefault(cio)
	if err != nil {
		return err
	}

	typeIDs := make([]string, 0)
	for _, d := range module.Types.Definitions() {
		typeIDs = append(typeIDs, string(d.ID))
	}

	var moduleNames []string
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		repoRoot, _, gitErr := gitutil.FindGitRoot(cwd)
		if gitErr == nil {
			dirs, dirErr := gitutil.FindAllChangesDirs(repoRoot)
			if dirErr == nil && len(dirs) > 0 {
				moduleMap, mapErr := changeentry.ResolveModulesForChangesDirs(repoRoot, dirs)
				if mapErr == nil {
					for _, dir := range dirs {
						m := moduleMap[dir]
						name := m.Name
						if name == "" {
							name = "(root)"
						}
						moduleNames = append(moduleNames, name)
					}
				}
			}
		}
	}
	if moduleNames == nil {
		moduleNames = []string{}
	}

	doc := struct {
		SchemaVersion int `json:"schema_version"`
		Defaults      struct {
			Audience  []string `json:"audience"`
			Rank      int      `json:"rank"`
			Component []string `json:"component"`
		} `json:"defaults"`
		GitWrite struct {
			Allow            bool `json:"allow"`
			AddChange        bool `json:"add_change"`
			CreateReleaseTag bool `json:"create_release_tag"`
			PushReleaseTag   bool `json:"push_release_tag"`
		} `json:"git_write"`
		Release struct {
			VPrefix   string `json:"v_prefix"`
			AliasTags string `json:"alias_tags"`
		} `json:"release"`
		Types   []string `json:"types"`
		Modules []string `json:"modules"`
	}{SchemaVersion: 1}

	doc.Defaults.Audience = nonNilSlice(module.Defaults.Audience)
	doc.Defaults.Rank = module.Defaults.Rank
	doc.Defaults.Component = nonNilSlice(module.Defaults.Component)
	doc.GitWrite.Allow = module.GitWrite.Enabled
	doc.GitWrite.AddChange = module.GitWrite.Add
	doc.GitWrite.CreateReleaseTag = module.GitWrite.ReleaseTag
	doc.GitWrite.PushReleaseTag = module.GitWrite.ReleasePush
	doc.Release.VPrefix = module.Release.VPrefix
	doc.Release.AliasTags = module.Release.AliasTags
	doc.Types = typeIDs
	doc.Modules = moduleNames

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// ── get ───────────────────────────────────────────────────────────────────────

func configGet(key string, cio changeentry.ConfigIO, w io.Writer) error {
	module, err := resolveModuleOrDefault(cio)
	if err != nil {
		return err
	}

	kd, ok := configKeyDefs[key]
	if !ok {
		return changeentry.NewValidationError("key", fmt.Sprintf("unknown config key %q; run 'chagg config --list' to see all keys", key))
	}

	fmt.Fprintln(w, kd.getResolved(module))
	return nil
}

// ── set ───────────────────────────────────────────────────────────────────────

func configSet(key string, values []string, global bool, cio changeentry.ConfigIO, quiet bool) error {
	kd, ok := configKeyDefs[key]
	if !ok {
		return changeentry.NewValidationError("key", fmt.Sprintf("unknown config key %q; run 'chagg config --list' to see all keys", key))
	}

	if global {
		cfg, err := cio.ReadUserConfig()
		if err != nil {
			return err
		}
		if cfg == nil {
			cfg = &changeentry.RawConfig{}
		}
		if err := kd.setRaw(cfg, values); err != nil {
			return err
		}
		if err := cio.WriteUserConfig(cfg); err != nil {
			return err
		}
		if !quiet {
			path, _ := cio.UserConfigPath()
			fmt.Printf("Set %s = %s (in %s)\n", key, strings.Join(values, ", "), path)
		}
		return nil
	}

	// Repo scope.
	repoRoot, err := requireGitRoot()
	if err != nil {
		return err
	}
	cfg, _, err := cio.ReadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &changeentry.RawConfig{}
	}
	if err := kd.setRaw(cfg, values); err != nil {
		return err
	}
	name, err := cio.WriteRepoConfig(repoRoot, cfg)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Set %s = %s (in %s)\n", key, strings.Join(values, ", "), name)
	}
	return nil
}

// ── unset ─────────────────────────────────────────────────────────────────────

func configUnset(key string, global bool, cio changeentry.ConfigIO, quiet bool) error {
	kd, ok := configKeyDefs[key]
	if !ok {
		return changeentry.NewValidationError("key", fmt.Sprintf("unknown config key %q", key))
	}

	if global {
		cfg, err := cio.ReadUserConfig()
		if err != nil {
			return err
		}
		if cfg == nil {
			cfg = &changeentry.RawConfig{}
		}
		kd.unsetRaw(cfg)
		if err := cio.WriteUserConfig(cfg); err != nil {
			return err
		}
		if !quiet {
			path, _ := cio.UserConfigPath()
			fmt.Printf("Unset %s (in %s)\n", key, path)
		}
		return nil
	}

	repoRoot, err := requireGitRoot()
	if err != nil {
		return err
	}
	cfg, _, err := cio.ReadRepoConfig(repoRoot)
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &changeentry.RawConfig{}
	}
	kd.unsetRaw(cfg)
	name, err := cio.WriteRepoConfig(repoRoot, cfg)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Printf("Unset %s (in %s)\n", key, name)
	}
	return nil
}

// ── key registry ──────────────────────────────────────────────────────────────

type configKeyDef struct {
	getResolved func(m changeentry.ModuleConfig) string
	setRaw      func(cfg *changeentry.RawConfig, values []string) error
	unsetRaw    func(cfg *changeentry.RawConfig)
}

var configKeyDefs = map[string]configKeyDef{
	"defaults.audience": {
		getResolved: func(m changeentry.ModuleConfig) string { return formatStringList(m.Defaults.Audience) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			cfg.Defaults.Audience = changeentry.StringListConfig(expandCSV(values))
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Defaults.Audience = nil },
	},
	"defaults.rank": {
		getResolved: func(m changeentry.ModuleConfig) string { return strconv.Itoa(m.Defaults.Rank) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			if len(values) != 1 {
				return changeentry.NewValidationError("defaults.rank", "expected exactly one integer value")
			}
			n, err := strconv.Atoi(values[0])
			if err != nil {
				return changeentry.NewValidationError("defaults.rank", fmt.Sprintf("%q is not a valid integer", values[0]))
			}
			cfg.Defaults.Rank = &n
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Defaults.Rank = nil },
	},
	"defaults.component": {
		getResolved: func(m changeentry.ModuleConfig) string { return formatStringList(m.Defaults.Component) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			cfg.Defaults.Component = changeentry.StringListConfig(expandCSV(values))
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Defaults.Component = nil },
	},
	"git.write.allow": {
		getResolved: func(m changeentry.ModuleConfig) string { return strconv.FormatBool(m.GitWrite.Enabled) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			b, err := parseBool(values)
			if err != nil {
				return changeentry.NewValidationError("git.write.allow", err.Error())
			}
			cfg.Git.Write.Allow = &b
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Git.Write.Allow = nil },
	},
	"git.write.add-change": {
		getResolved: func(m changeentry.ModuleConfig) string { return strconv.FormatBool(m.GitWrite.Add) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			b, err := parseBool(values)
			if err != nil {
				return changeentry.NewValidationError("git.write.add-change", err.Error())
			}
			cfg.Git.Write.Operations.AddChange = &b
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Git.Write.Operations.AddChange = nil },
	},
	"git.write.create-release-tag": {
		getResolved: func(m changeentry.ModuleConfig) string { return strconv.FormatBool(m.GitWrite.ReleaseTag) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			b, err := parseBool(values)
			if err != nil {
				return changeentry.NewValidationError("git.write.create-release-tag", err.Error())
			}
			cfg.Git.Write.Operations.CreateReleaseTag = &b
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Git.Write.Operations.CreateReleaseTag = nil },
	},
	"git.write.push-release-tag": {
		getResolved: func(m changeentry.ModuleConfig) string { return strconv.FormatBool(m.GitWrite.ReleasePush) },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			b, err := parseBool(values)
			if err != nil {
				return changeentry.NewValidationError("git.write.push-release-tag", err.Error())
			}
			cfg.Git.Write.Operations.PushReleaseTag = &b
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Git.Write.Operations.PushReleaseTag = nil },
	},
	"release.v-prefix": {
		getResolved: func(m changeentry.ModuleConfig) string { return m.Release.VPrefix },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			if len(values) != 1 {
				return changeentry.NewValidationError("release.v-prefix", "expected one value: auto, always, or never")
			}
			v := strings.ToLower(strings.TrimSpace(values[0]))
			if v != "auto" && v != "always" && v != "never" {
				return changeentry.NewValidationError("release.v-prefix",
					fmt.Sprintf("%q is not valid; use auto, always, or never", values[0]))
			}
			cfg.Release.VPrefix = v
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Release.VPrefix = "" },
	},
	"release.alias-tags": {
		getResolved: func(m changeentry.ModuleConfig) string { return m.Release.AliasTags },
		setRaw: func(cfg *changeentry.RawConfig, values []string) error {
			if len(values) != 1 {
				return changeentry.NewValidationError("release.alias-tags", "expected one value: auto, always, or never")
			}
			v := strings.ToLower(strings.TrimSpace(values[0]))
			if v != "auto" && v != "always" && v != "never" {
				return changeentry.NewValidationError("release.alias-tags",
					fmt.Sprintf("%q is not valid; use auto, always, or never", values[0]))
			}
			cfg.Release.AliasTags = v
			return nil
		},
		unsetRaw: func(cfg *changeentry.RawConfig) { cfg.Release.AliasTags = "" },
	},
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveModuleOrDefault(cio changeentry.ConfigIO) (changeentry.ModuleConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return changeentry.ModuleConfig{}, fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, _, err := gitutil.FindGitRoot(cwd)
	if err != nil {
		return changeentry.ModuleConfig{}, err
	}
	changesDir, err := changeentry.ResolveChangesDirectory(cwd)
	if err != nil {
		return changeentry.ModuleConfig{}, err
	}
	return changeentry.ResolveModuleForChangesDir(repoRoot, changesDir)
}

func requireGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	repoRoot, _, err := gitutil.FindGitRoot(cwd)
	return repoRoot, err
}

func formatStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ", ")
}

func expandCSV(values []string) []string {
	var result []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if t := strings.TrimSpace(part); t != "" {
				result = append(result, t)
			}
		}
	}
	return result
}

func parseBool(values []string) (bool, error) {
	if len(values) != 1 {
		return false, fmt.Errorf("expected exactly one boolean value (true or false)")
	}
	b, err := strconv.ParseBool(values[0])
	if err != nil {
		return false, fmt.Errorf("%q is not a valid boolean (use true or false)", values[0])
	}
	return b, nil
}
