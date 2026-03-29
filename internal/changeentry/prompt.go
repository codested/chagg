package changeentry

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// promptString prints label to output and reads a single line from reader.
// When the user enters a blank line, defaultValue is returned.
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
		trimmed := strings.TrimSpace(params.Bump)
		if trimmed != "" {
			return NormalizeBumpLevel(trimmed)
		}
		// --bump passed without a value — prompt if interactive.
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

	// Flag not passed — use type default silently.
	return "", nil
}

func resolveStringList(flagValue string, isSet bool, reader *bufio.Reader, output io.Writer, interactive bool, label string, defaultValue []string) ([]string, error) {
	if isSet {
		values := parseCSVList(flagValue)
		if len(values) > 0 {
			return values, nil
		}
		// Flag present without value — prompt if interactive, otherwise use defaults.
		if !interactive {
			return defaultValue, nil
		}
		defaultText := strings.Join(defaultValue, ",")
		value, err := promptString(reader, output, label+": ", defaultText)
		if err != nil {
			return nil, err
		}
		values = parseCSVList(value)
		if len(values) == 0 {
			return defaultValue, nil
		}
		return values, nil
	}

	// Flag not passed at all — use defaults silently.
	return defaultValue, nil
}

func resolveRank(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (int, error) {
	if params.RankSet {
		if params.Rank != 0 {
			return params.Rank, nil
		}
		// --rank passed without a value — prompt if interactive.
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

	// Flag not passed — use default silently.
	return params.Defaults.Rank, nil
}

func resolveRelease(params Params, reader *bufio.Reader, output io.Writer, interactive bool) (string, error) {
	if params.ReleaseSet {
		trimmed := strings.TrimSpace(params.Release)
		if trimmed != "" {
			return trimmed, nil
		}
		// --release passed without a value — prompt if interactive.
		if !interactive {
			return "", nil
		}
		return promptString(reader, output, "Release version (optional): ", "")
	}

	// Flag not passed — skip silently.
	return "", nil
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
