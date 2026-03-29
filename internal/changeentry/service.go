package changeentry

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Type         string
	TypeSet      bool
	Bump         string
	BumpSet      bool
	Component    string
	ComponentSet bool
	Audience     string
	AudienceSet  bool
	Rank         int
	RankSet      bool
	Issue        string
	IssueSet     bool
	Release      string
	ReleaseSet   bool
	Body         string
	BodySet      bool
	Defaults     Defaults
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
	inferArg := targetArg
	if !strings.HasSuffix(strings.ToLower(filepath.Base(inferArg)), ".md") {
		inferArg += ".md"
	}
	if inferredType, err := InferTypeFromFilename(inferArg, registry); err == nil {
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
