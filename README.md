![chagg](README.logo.png)

`chagg` is a release-note workflow for Git repositories.

Instead of writing changelogs at release time, you collect small change entry files in `.changes/`. `chagg` then
validates them, shows release previews, generates Markdown changelogs, and creates the next release tag.

**Full documentation at [chagg.dev/docs](https://chagg.dev/docs)**

## Installation

**Quick install** (Linux, macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh
```

<details>
<summary><strong>Other methods</strong></summary>

**Pin a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh -s -- --version v1.2.3
```

**Custom install directory** (default: `/usr/local/bin`):

```bash
curl -fsSL https://raw.githubusercontent.com/codested/chagg/main/install.sh | sh -s -- --dir ~/.local/bin
```

**From source** (requires Go 1.23+):

```bash
go install github.com/codested/chagg/cmd/chagg@latest
```

**Manual download:**

Download the binary for your platform from the [releases page](https://github.com/codested/chagg/releases), make it executable, and move it to a directory in your `PATH`.

</details>

## Quick start

```bash
chagg init                   # one-time setup — creates .changes/
chagg add auth/new-login     # create a change entry
chagg check                  # validate all entries
chagg log                    # preview staging changes
chagg generate > CHANGELOG.md  # generate markdown changelog
chagg release                # create the next version tag
```

## Typical release flow

```bash
# 1) Add entries while developing
chagg add feat__oauth-login --body "Add OAuth login." --no-prompt
chagg add auth/fix__session-timeout --type fix --bump minor

# 2) Validate and preview
chagg check
chagg log

# 3) Generate and release
chagg generate > CHANGELOG.md
chagg release --push
```

## Scripting and piping

All commands support `--format json` for machine-readable output. JSON includes `schema_version` for forward-compatible parsing.

```bash
chagg generate --format json | jq '.groups[].types[].entries[].preview'
chagg log --format json | jq '.next_tag'
chagg check --format json | jq '.summary'
chagg config --format json | jq '.git_write'
VERSION=$(chagg release --dry-run --version-only)
echo "Long description" | chagg add feat__foo --body - --bump minor
```

Use `-q` / `--quiet` to suppress informational messages in scripts.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error (I/O failure, git error) |
| `2` | Validation error (malformed entry, invalid flag) |
| `3` | Conflict (tag already exists, uncommitted changes) |

## Documentation

| Topic | Link |
|-------|------|
| Getting Started | [chagg.dev/docs/getting-started](https://chagg.dev/docs/getting-started) |
| Installation | [chagg.dev/docs/installation](https://chagg.dev/docs/installation) |
| Configuration | [chagg.dev/docs/configuration](https://chagg.dev/docs/configuration) |
| Change Entries | [chagg.dev/docs/change-entries](https://chagg.dev/docs/change-entries) |
| CI Integration | [chagg.dev/docs/ci-integration](https://chagg.dev/docs/ci-integration) |
| Multi-Module Repos | [chagg.dev/docs/multi-module](https://chagg.dev/docs/multi-module) |
| Command Reference | [chagg.dev/docs/commands](https://chagg.dev/docs/commands) |
| Web Dashboard | [chagg.dev/docs/web-dashboard](https://chagg.dev/docs/web-dashboard) |

## GitHub Action

The [`chagg-github-actions`](https://github.com/codested/chagg-github-actions) provides two composite actions:

- **CI** — validates entries on every push/PR, posts changelog preview comments
- **Release** — generates changelogs on version tag push, exposes `changelog`, `version`, and `module_dir` outputs

## Community

- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Report an issue](https://github.com/codested/chagg/issues)
