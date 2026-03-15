package changeentry

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Entry struct {
	Type      ChangeType
	Bump      BumpLevel
	Component []string
	Audience  []string
	Rank      int
	Issue     []string
	Release   string
	Body      string
}

// Params carries CLI-provided values for a new change entry.
// Defaults carries the resolved per-module defaults used when a field is not
// explicitly set.
type Params struct {
	Type        string
	TypeSet     bool
	Bump        string
	BumpSet     bool
	Component   string
	ComponentSet bool
	Audience    string
	AudienceSet bool
	Rank        int
	RankSet     bool
	Issue       string
	IssueSet    bool
	Release     string
	ReleaseSet  bool
	Body        string
	BodySet     bool
	Defaults    Defaults
}

// CreateChange creates a new change entry file under module.ChangesDir.
// The caller is responsible for ensuring the directory already exists.
func CreateChange(module ModuleConfig, targetArg string, params Params, input io.Reader, output io.Writer, interactive bool) (string, error) {
	reader := bufio.NewReader(input)

	resolvedTargetArg, err := resolveTargetPathArg(targetArg, reader, output, interactive)
	if err != nil {
		return "", err
	}

	resolvedTargetArg, inferredType, err := resolveTypedTargetPath(resolvedTargetArg, module.Types, params, reader, output, interactive)
	if err != nil {
		return "", err
	}

	targetPath, err := BuildChangeFilePath(module.ChangesDir, resolvedTargetArg)
	if err != nil {
		return "", err
	}

	if _, err = os.Stat(targetPath); err == nil {
		return "", NewConflictError(fmt.Sprintf("change file already exists: %s", targetPath))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err = os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}

	entry, err := collectEntry(module, params, inferredType, reader, output, interactive)
	if err != nil {
		return "", err
	}

	content, err := RenderEntry(entry)
	if err != nil {
		return "", err
	}

	if err = os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return "", err
	}

	return targetPath, nil
}

func resolveTypedTargetPath(targetArg string, registry TypeRegistry, params Params, reader *bufio.Reader, output io.Writer, interactive bool) (string, ChangeType, error) {
	if inferredType, err := InferTypeFromFilename(targetArg, registry); err == nil {
		return targetArg, inferredType, nil
	}

	resolvedType, err := resolveType(registry, params, reader, output, interactive)
	if err != nil {
		return "", "", err
	}

	clean := filepath.Clean(targetArg)
	dir := filepath.Dir(clean)
	base := filepath.Base(clean)
	typedBase := fmt.Sprintf("%s__%s", resolvedType, base)
	if dir == "." {
		return typedBase, resolvedType, nil
	}

	return filepath.Join(dir, typedBase), resolvedType, nil
}

func resolveTargetPathArg(targetArg string, reader *bufio.Reader, output io.Writer, interactive bool) (string, error) {
	trimmedTarget := strings.TrimSpace(targetArg)
	if trimmedTarget != "" {
		return trimmedTarget, nil
	}

	if !interactive {
		return "", NewValidationError("path", "missing target path (example: chagg add auth/new-login)")
	}

	for {
		value, err := promptString(reader, output, "Path (example: auth/new-login): ", "")
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(value) == "" {
			_, _ = fmt.Fprintln(output, "Path is required")
			continue
		}

		return value, nil
	}
}

func ResolveChangesDirectory(startPath string) (string, error) {
	current, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		changesPath := filepath.Join(current, ".changes")
		changesInfo, changesErr := os.Stat(changesPath)
		if changesErr == nil {
			if !changesInfo.IsDir() {
				return "", NewValidationError("path", fmt.Sprintf("%s exists but is not a directory", changesPath))
			}
			return changesPath, nil
		}
		if !errors.Is(changesErr, os.ErrNotExist) {
			return "", changesErr
		}

		gitPath := filepath.Join(current, ".git")
		if _, gitErr := os.Stat(gitPath); gitErr == nil {
			return changesPath, nil
		} else if !errors.Is(gitErr, os.ErrNotExist) {
			return "", gitErr
		}

		parent := filepath.Dir(current)
		if parent == current {
			return changesPath, nil
		}

		current = parent
	}
}

func BuildChangeFilePath(changesDir string, targetArg string) (string, error) {
	if targetArg == "" {
		return "", NewValidationError("path", "target path cannot be empty")
	}

	clean := filepath.Clean(targetArg)
	if filepath.IsAbs(clean) {
		return "", NewValidationError("path", fmt.Sprintf("target path must be relative: %s", targetArg))
	}

	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", NewValidationError("path", fmt.Sprintf("target path must not escape .changes: %s", targetArg))
	}

	if !strings.HasSuffix(strings.ToLower(clean), ".md") {
		clean += ".md"
	}

	return filepath.Join(changesDir, clean), nil
}

func collectEntry(module ModuleConfig, params Params, inferredType ChangeType, reader *bufio.Reader, output io.Writer, interactive bool) (Entry, error) {
	bump, err := resolveBump(params, reader, output, interactive)
	if err != nil {
		return Entry{}, err
	}

	component, err := resolveStringList(params.Component, params.ComponentSet, reader, output, interactive, "Component(s), comma separated", params.Defaults.Component)
	if err != nil {
		return Entry{}, err
	}

	audience, err := resolveStringList(params.Audience, params.AudienceSet, reader, output, interactive, "Audience(s), comma separated", params.Defaults.Audience)
	if err != nil {
		return Entry{}, err
	}

	rank, err := resolveRank(params, reader, output, interactive)
	if err != nil {
		return Entry{}, err
	}

	issue, err := resolveStringList(params.Issue, params.IssueSet, reader, output, interactive, "Issue(s), comma separated", nil)
	if err != nil {
		return Entry{}, err
	}

	release, err := resolveRelease(params, reader, output, interactive)
	if err != nil {
		return Entry{}, err
	}

	body, err := resolveBody(params, reader, output, interactive)
	if err != nil {
		return Entry{}, err
	}

	return Entry{
		Type:      inferredType,
		Bump:      bump,
		Component: component,
		Audience:  audience,
		Rank:      rank,
		Issue:     issue,
		Release:   release,
		Body:      body,
	}, nil
}

func resolveType(registry TypeRegistry, params Params, reader *bufio.Reader, output io.Writer, interactive bool) (ChangeType, error) {
	if params.TypeSet {
		flagValue := strings.TrimSpace(params.Type)
		if flagValue != "" {
			return registry.NormalizeType(flagValue)
		}
	}

	if !interactive {
		return "", NewValidationError("type", "--type is required when running non-interactively")
	}

	for {
		value, err := promptString(reader, output, registry.TypePrompt(), "")
		if err != nil {
			return "", err
		}

		normalized, err := registry.NormalizeType(value)
		if err != nil {
			_, _ = fmt.Fprintln(output, err)
			continue
		}
		return normalized, nil
	}
}

func resolveBump(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (BumpLevel, error) {
	if params.BumpSet {
		return NormalizeBumpLevel(params.Bump)
	}

	if !interactive {
		return "", nil
	}

	for {
		value, err := promptString(reader, output, BumpPrompt(), "")
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(value) == "" {
			return "", nil
		}

		level, err := NormalizeBumpLevel(value)
		if err != nil {
			_, _ = fmt.Fprintln(output, err)
			continue
		}

		return level, nil
	}
}

func resolveStringList(flagValue string, isSet bool, reader *bufio.Reader, output io.Writer, interactive bool, label string, defaultValue []string) ([]string, error) {
	if isSet {
		values := parseCSVList(flagValue)
		if len(values) == 0 {
			return defaultValue, nil
		}
		return values, nil
	}

	if !interactive {
		return defaultValue, nil
	}

	defaultText := strings.Join(defaultValue, ",")
	value, err := promptString(reader, output, label+": ", defaultText)
	if err != nil {
		return nil, err
	}

	values := parseCSVList(value)
	if len(values) == 0 {
		return defaultValue, nil
	}

	return values, nil
}

func resolveRank(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (int, error) {
	if params.RankSet {
		return params.Rank, nil
	}

	if !interactive {
		return params.Defaults.Rank, nil
	}

	defaultText := strconv.Itoa(params.Defaults.Rank)
	for {
		value, err := promptString(reader, output, "Rank (higher numbers are shown first): ", defaultText)
		if err != nil {
			return 0, err
		}

		rank, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			_, _ = fmt.Fprintln(output, "Rank must be an integer")
			continue
		}

		return rank, nil
	}
}

func resolveRelease(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (string, error) {
	if params.ReleaseSet {
		return strings.TrimSpace(params.Release), nil
	}

	if !interactive {
		return "", nil
	}

	return promptString(reader, output, "Release version (optional): ", "")
}

func resolveBody(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (string, error) {
	if params.BodySet {
		return strings.TrimSpace(params.Body), nil
	}

	if !interactive {
		return "", nil
	}

	_, _ = fmt.Fprintln(output, "Body (finish with a single '.' on a line):")

	lines := make([]string, 0)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		if strings.TrimSpace(line) == "." {
			break
		}
		lines = append(lines, line)

		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func promptString(reader *bufio.Reader, output io.Writer, label string, defaultValue string) (string, error) {
	if defaultValue == "" {
		_, _ = fmt.Fprint(output, label)
	} else {
		_, _ = fmt.Fprintf(output, "%s[%s]: ", label, defaultValue)
	}

	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}

	return value, nil
}

func parseCSVList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parts := strings.Split(trimmed, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}

	return result
}