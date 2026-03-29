package version

import "testing"

func TestFull_defaults(t *testing.T) {
	// Reset to defaults for the test.
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "dev"
	Commit = "unknown"
	Date = "unknown"

	got := Full()
	if got != "dev" {
		t.Errorf("Full() with defaults = %q, want %q", got, "dev")
	}
}

func TestFull_withBuildInfo(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "1.2.3"
	Commit = "abc1234def5678"
	Date = "2026-03-27T12:00:00Z"

	got := Full()
	want := "1.2.3 (abc1234, 2026-03-27T12:00:00Z)"
	if got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}

func TestFull_commitOnly(t *testing.T) {
	origVersion, origCommit, origDate := Version, Commit, Date
	defer func() { Version, Commit, Date = origVersion, origCommit, origDate }()

	Version = "0.11.0"
	Commit = "abcdef1"
	Date = "unknown"

	got := Full()
	want := "0.11.0 (abcdef1)"
	if got != want {
		t.Errorf("Full() = %q, want %q", got, want)
	}
}
