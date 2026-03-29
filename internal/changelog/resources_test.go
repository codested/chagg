package changelog

import (
	"path/filepath"
	"testing"
)

func TestRewriteImagePaths(t *testing.T) {
	// Simulate: entry at /repo/.changes/feature/2026/change.md, CWD = /repo
	entryPath := filepath.Join("/", "repo", ".changes", "feature", "2026", "change.md")
	baseDir := filepath.Join("/", "repo")

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "relative image same dir",
			body: "![screenshot](change.jpg)",
			want: "![screenshot](.changes/feature/2026/change.jpg)",
		},
		{
			name: "relative image subdirectory",
			body: "![diagram](images/arch.png)",
			want: "![diagram](.changes/feature/2026/images/arch.png)",
		},
		{
			name: "URL unchanged",
			body: "![logo](https://example.com/logo.png)",
			want: "![logo](https://example.com/logo.png)",
		},
		{
			name: "absolute path unchanged",
			body: "![pic](/tmp/pic.png)",
			want: "![pic](/tmp/pic.png)",
		},
		{
			name: "multiple images",
			body: "Before ![a](a.png) middle ![b](b.png) after",
			want: "Before ![a](.changes/feature/2026/a.png) middle ![b](.changes/feature/2026/b.png) after",
		},
		{
			name: "no images",
			body: "Just some text with [a link](https://example.com)",
			want: "Just some text with [a link](https://example.com)",
		},
		{
			name: "empty alt text",
			body: "![](diagram.svg)",
			want: "![](.changes/feature/2026/diagram.svg)",
		},
		{
			name: "parent directory traversal",
			body: "![img](../shared/logo.png)",
			want: "![img](.changes/feature/shared/logo.png)",
		},
		{
			name: "protocol-relative URL",
			body: "![img](//cdn.example.com/img.png)",
			want: "![img](//cdn.example.com/img.png)",
		},
		{
			name: "image with title",
			body: `![photo](photo.jpg "A nice photo")`,
			want: `![photo](.changes/feature/2026/photo.jpg "A nice photo")`,
		},
		{
			name: "image with empty title",
			body: `![photo](photo.jpg "")`,
			want: `![photo](.changes/feature/2026/photo.jpg "")`,
		},
		{
			name: "image with single-quoted title",
			body: `![photo](photo.jpg 'A nice photo')`,
			want: `![photo](.changes/feature/2026/photo.jpg 'A nice photo')`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteImagePaths(tt.body, entryPath, baseDir)
			if got != tt.want {
				t.Errorf("rewriteImagePaths() =\n  %q\nwant:\n  %q", got, tt.want)
			}
		})
	}
}

func TestCollectResources(t *testing.T) {
	entryPath := filepath.Join("/", "repo", ".changes", "feat", "change.md")
	baseDir := filepath.Join("/", "repo")

	body := "![screenshot](shot.png)\n![logo](https://example.com/logo.png)\n![diagram](images/arch.svg)\n![titled](photo.jpg \"A title\")\n![single](icon.png 'Icon')"
	resources := collectResources(body, entryPath, "change@abc1234", baseDir)

	if len(resources) != 4 {
		t.Fatalf("expected 4 resources, got %d", len(resources))
	}

	// First: shot.png
	if resources[0].RelPath != ".changes/feat/shot.png" {
		t.Errorf("resources[0].RelPath = %q, want %q", resources[0].RelPath, ".changes/feat/shot.png")
	}
	if resources[0].EntryID != "change@abc1234" {
		t.Errorf("resources[0].EntryID = %q, want %q", resources[0].EntryID, "change@abc1234")
	}

	// Second: images/arch.svg
	if resources[1].RelPath != ".changes/feat/images/arch.svg" {
		t.Errorf("resources[1].RelPath = %q, want %q", resources[1].RelPath, ".changes/feat/images/arch.svg")
	}

	// Third: photo.jpg (from titled image syntax)
	if resources[2].RelPath != ".changes/feat/photo.jpg" {
		t.Errorf("resources[2].RelPath = %q, want %q", resources[2].RelPath, ".changes/feat/photo.jpg")
	}

	// Fourth: icon.png (from single-quoted title syntax)
	if resources[3].RelPath != ".changes/feat/icon.png" {
		t.Errorf("resources[3].RelPath = %q, want %q", resources[3].RelPath, ".changes/feat/icon.png")
	}
}

func TestIsURLOrAbsolute(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"https://example.com/img.png", true},
		{"http://example.com/img.png", true},
		{"//cdn.example.com/img.png", true},
		{"/absolute/path.png", true},
		{"relative/path.png", false},
		{"image.jpg", false},
		{"../up/image.jpg", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isURLOrAbsolute(tt.path); got != tt.want {
				t.Errorf("isURLOrAbsolute(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
