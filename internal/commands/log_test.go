package commands

import (
	"testing"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
)

func TestComputeVersionHintsNoTagsNoStaging(t *testing.T) {
	cl := &changelog.ChangeLog{Module: changeentry.ModuleConfig{Name: "default"}}

	latest, next := computeVersionHints(changeentry.ModuleConfig{}, nil, cl)
	if latest != "none" {
		t.Fatalf("expected latest none, got %q", latest)
	}
	if next != "none (no staging changes)" {
		t.Fatalf("expected no staging next text, got %q", next)
	}
}

func TestComputeVersionHintsNoTagsWithStaging(t *testing.T) {
	cl := &changelog.ChangeLog{
		Groups: []changelog.VersionGroup{{
			Version: "staging",
			TypeGroups: []changelog.TypeGroup{{
				Entries: []changelog.EntryWithMeta{{
					Entry: changeentry.Entry{Type: changeentry.ChangeTypeFix},
				}},
			}},
		}},
	}

	latest, next := computeVersionHints(changeentry.ModuleConfig{TagPrefix: "msal-react-"}, nil, cl)
	if latest != "none" {
		t.Fatalf("expected latest none, got %q", latest)
	}
	if next != "msal-react-0.1.0" {
		t.Fatalf("expected msal-react-0.1.0, got %q", next)
	}
}

func TestComputeVersionHintsWithLatestTagAndStaging(t *testing.T) {
	tags := []changelog.Tag{{
		Name:       "v1.2.3",
		Version:    changelog.SemVersion{Major: 1, Minor: 2, Patch: 3},
		HasVPrefix: true,
	}}
	cl := &changelog.ChangeLog{
		Groups: []changelog.VersionGroup{{
			Version: "staging",
			TypeGroups: []changelog.TypeGroup{{
				Entries: []changelog.EntryWithMeta{{
					Entry: changeentry.Entry{Type: changeentry.ChangeTypeFeature},
				}},
			}},
		}},
	}

	latest, next := computeVersionHints(changeentry.ModuleConfig{}, tags, cl)
	if latest != "v1.2.3" {
		t.Fatalf("expected latest v1.2.3, got %q", latest)
	}
	if next != "v1.3.0" {
		t.Fatalf("expected v1.3.0, got %q", next)
	}
}
