package semver

import "testing"

func TestParseSemVersionSupportsPreReleaseAndBuild(t *testing.T) {
	parsed, hasV, err := ParseSemVersion("v1.2.3-beta.1+build.42")
	if err != nil {
		t.Fatalf("ParseSemVersion returned error: %v", err)
	}

	if !hasV {
		t.Fatalf("expected hasV to be true")
	}

	if parsed.Major != 1 || parsed.Minor != 2 || parsed.Patch != 3 {
		t.Fatalf("unexpected core version: %+v", parsed)
	}

	if parsed.PreRelease != "beta.1" {
		t.Fatalf("expected pre-release beta.1, got %q", parsed.PreRelease)
	}

	if parsed.Build != "build.42" {
		t.Fatalf("expected build build.42, got %q", parsed.Build)
	}
}

func TestCompareSemVersionTreatsStableAsHigherThanPreRelease(t *testing.T) {
	stable := SemVersion{Major: 1, Minor: 2, Patch: 3}
	pre := SemVersion{Major: 1, Minor: 2, Patch: 3, PreRelease: "beta.1"}

	if CompareSemVersion(stable, pre) <= 0 {
		t.Fatalf("expected stable > pre-release")
	}
}

func TestNextPreReleaseLabelVersionIncrementsCounter(t *testing.T) {
	base := SemVersion{Major: 2, Minor: 0, Patch: 0}
	tags := []Tag{
		{Name: "v2.0.0-beta.1", Version: SemVersion{Major: 2, Minor: 0, Patch: 0, PreRelease: "beta.1"}},
		{Name: "v2.0.0-beta.2", Version: SemVersion{Major: 2, Minor: 0, Patch: 0, PreRelease: "beta.2"}},
	}

	next := NextPreReleaseLabelVersion(base, "beta", tags)
	if next.PreRelease != "beta.3" {
		t.Fatalf("expected beta.3, got %q", next.PreRelease)
	}
}
