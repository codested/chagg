package changelog

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Resource represents a non-markdown file referenced by a change entry body
// (e.g. an image). AbsPath is the resolved location on disk; RelPath is the
// path relative to the caller's working directory.
type Resource struct {
	AbsPath string `json:"abs_path"`
	RelPath string `json:"rel_path"`
	EntryID string `json:"entry_id"`
}

// markdownImageRe matches markdown image syntax: ![alt](path) or ![alt](path "title")
// It captures the alt text (group 1), the path (group 2), and the optional title
// including surrounding quotes and leading space (group 3).
var markdownImageRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^\s)]+)(\s+["'][^"']*["'])?\)`)

// rewriteImagePaths rewrites relative image paths in a markdown body so they
// are relative to baseDir (typically the caller's CWD) instead of the entry
// file's directory.  Absolute paths and URLs are left unchanged.
func rewriteImagePaths(body string, entryPath string, baseDir string) string {
	entryDir := filepath.Dir(entryPath)

	return markdownImageRe.ReplaceAllStringFunc(body, func(match string) string {
		sub := markdownImageRe.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		alt, imgPath := sub[1], sub[2]

		if isURLOrAbsolute(imgPath) {
			return match
		}

		abs := filepath.Join(entryDir, imgPath)
		rel, err := filepath.Rel(baseDir, abs)
		if err != nil {
			return match
		}

		title := ""
		if len(sub) > 3 {
			title = sub[3] // includes leading space + quotes, e.g. ` "A title"`
		}
		return "![" + alt + "](" + filepath.ToSlash(rel) + title + ")"
	})
}

// collectResources extracts all non-URL, non-absolute image references from a
// body and returns them as Resource entries resolved against baseDir.
func collectResources(body string, entryPath string, entryID string, baseDir string) []Resource {
	entryDir := filepath.Dir(entryPath)
	matches := markdownImageRe.FindAllStringSubmatch(body, -1)

	var resources []Resource
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		imgPath := m[2]
		if isURLOrAbsolute(imgPath) {
			continue
		}

		abs := filepath.Join(entryDir, imgPath)
		rel, err := filepath.Rel(baseDir, abs)
		if err != nil {
			continue
		}

		resources = append(resources, Resource{
			AbsPath: abs,
			RelPath: filepath.ToSlash(rel),
			EntryID: entryID,
		})
	}
	return resources
}

func isURLOrAbsolute(path string) bool {
	return filepath.IsAbs(path) ||
		strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "//")
}
