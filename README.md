# chagg

`chagg` is a release-note workflow for Git repositories.

Instead of writing changelogs at release time, you collect small change entry files in `.changes/`. `chagg` then
validates them, shows release previews, generates Markdown changelogs, and creates the next release tag.

## Quick start

```bash
# create a change entry interactively
chagg add auth/new-login

# validate all change entries
chagg check

# preview unreleased (staging) changes
chagg log

# generate a markdown changelog
chagg generate > CHANGELOG.md

# create the next release tag
chagg release
```

## How `.changes` is resolved

For commands that operate on a single working changes directory (for example `add`, `log`, `generate`, `release`):

1. Start at the current working directory.
2. Walk upward until an existing `.changes` directory is found.
3. If none exists, keep walking to `.git` (or filesystem root) and create/use `.changes` there.

For `check`, `chagg` finds the Git root and validates **all** `.changes` directories below it (useful for multi-module
repositories).

## Multi-module configuration

For monorepos, add an optional `.chagg.yaml` at the Git root to override module names/tag prefixes when needed.

```yaml
modules:
  - name: msal-browser
    changes-dir: lib/msal-browser/.changes
    tag-prefix: msal-browser-
  - name: msal-node
    changes-dir: lib/msal-node/.changes
    tag-prefix: msal-node-
```

Rules:

- `changes-dir` is relative to repo root.
- `tag-prefix` is prepended when tags are read/created (for example `msal-browser-1.4.0`).
- `name` and `tag-prefix` are optional; when omitted they are inferred from the parent directory of `changes-dir`.
- Without config, `chagg` also infers modules from discovered `.changes` directories using the parent directory name.
- For repo-root `.changes`, the inferred `tag-prefix` is empty.
- If two discovered `.changes` directories infer the same module name, commands fail until you disambiguate with explicit `modules` entries.
- Supported config file names at repo root: `.chagg.yaml`, `.chagg.yml`, `chagg.yml`.

Optional global git-write policy:

```yaml
git-write:
  allow: false
```
```yaml
# OR granular controls:
git-write:
  allow:
    add-change: true
    push-release-tag: false
```

- `git-write.allow`: global kill-switch for write operations.
- `git-write.allow.add-change`: allow/disallow staging new change files.
- `git-write.allow.push-release-tag`: allow/disallow automatic `chagg release --push`.
- Omit `git-write` entirely to use built-in defaults (all allowed).

Optional default audience:

```yaml
default-audience: public
```

```yaml
default-audience:
  - public
  - developer
```

- `default-audience` is optional and can be a single string or a list.
- When omitted, entries without `audience` stay empty (no implicit audience).

## Change entry format

Change type is encoded in the filename prefix, not in front matter.

Filename schema (case-insensitive):

- `<type>__<title>.md` (preferred)
- `<type>_<title>.md` (also accepted)

Examples:

- `feature__oauth-login.md`
- `FEAT__quick-fix.md`
- `Feat_small-update.md`

Supported type prefixes are aliases of: `feature`, `fix`, `removal`, `security`, `docs`.

Front matter is optional and only needed for overrides.

```markdown
---
component:
  - api
audience: public
breaking: true
rank: 80
issue:
  - JIRA-410
release: v2.1.0
---

Add OAuth login support.
```

### Fields

- `breaking` (optional, default `false`)
- `component` (optional, string or list)
- `audience` (optional, string or list, defaults to `default-audience` when configured)
- `rank` (optional, default `0`; higher numbers are shown first)
- `issue` (optional, string or list)
- `release` (optional): pins this entry to a specific version
- Additional custom front-matter fields are allowed and ignored by `chagg`.

Notes:

- Default values (`breaking: false`, empty audience/default-audience, `rank: 0`) are omitted in newly rendered files.
- The body is free Markdown. `log` uses the first non-empty line as preview text.

## Commands

### `chagg add <path>`

Creates a new entry file below `.changes`.

- `chagg add auth/token-expiry --type fix` -> `.changes/auth/fix__token-expiry.md`
- Missing directories are created automatically.
- Supports flags for all entry properties (`--type`, `--breaking`, `--component`, `--audience`, `--rank`, `--issue`,
  `--release`, `--body`).
- `--rank` controls ordering in changelog output (higher values first).
- By default, new files are staged automatically (`git add`) after creation (built-in default).
- Use `--no-git-add` to skip staging, or `--git-add` to force staging explicitly.
- If the target filename already starts with a type prefix (for example `feat__login`), `--type` is optional.
- If no filename prefix is present, `--type` (or interactive prompt) is used to add the prefix automatically.
- If stdin is piped, prompts are skipped (no blocking).
- `--no-prompt` forces non-interactive mode (recommended for CI and AI tooling).

AI/automation example:

```bash
chagg add prototype/ai-generated-note \
  --type docs \
  --body "Generated by an AI assistant from merged PR summaries." \
  --no-prompt
```

### `chagg check`

Validates all change entry files in all discovered `.changes` directories.

- Verifies filename type prefix schema and supported front-matter values.
- Prints deterministic, module-grouped per-file validation results using repository-relative paths.
- Prints a valid/invalid summary.
- Returns non-zero exit code when invalid entries are found.

### `chagg log [version]`

Shows a human-friendly release preview.

- No argument: shows `staging` (changes since last SemVer tag).
- With `version` (for example `v1.4.0`): shows changes assigned to that release.
- Filters: `--audience`, `--component`, `--type`.
- `--preview-length <n>` controls preview truncation length (default: `80`).
- In multi-module mode, tags are scoped to the module's `tagPrefix`.
- If invalid change files are present, `log` fails and asks you to run `chagg check`.

Version assignment rules:

1. If `release:` is set, that pinned version is used.
2. Otherwise, the file is assigned by the commit where it was originally added.
3. Edits/moves do not change version assignment.

### `chagg generate`

Generates a changelog grouped by version and change type.

- Default: staging changes + the most recent tagged release (`-n 1 --show-staged`).
- `-n <count>`: number of tagged releases to include, newest first (default `1`, `0` = all).
- `--no-show-staged`: omit unreleased (staging) changes.
- `--since <version>`: include that version and all newer, plus staging.
- `--format <markdown|json>`: output format (default `markdown`).
- Filters: `--audience`, `--component`, `--type`.

Examples:

```bash
# default: staging + latest release
chagg generate

# all releases + staging
chagg generate -n 0

# latest release only, no staging
chagg generate --no-show-staged

# last 3 releases + staging
chagg generate -n 3

# all releases, no staging
chagg generate -n 0 --no-show-staged
```

### `chagg release`

Creates the next release tag from current staging changes.

- If no staging changes exist, nothing is tagged.
- If tags already exist, next SemVer is computed from staging entries:
    - major: any `breaking: true` or `type: removal`
    - minor: any `type: feature` (when no major)
    - patch: otherwise
- If no SemVer tag exists yet, prompts for the initial version (default `0.1.0`).
- Tag is created **locally only**. `chagg` prints a copy-paste command to push it.
- `--dry-run`: computes and prints what would happen, but does not create/push tags.
- `--version-only`: prints only the computed version (for scripts/export).
- `--push`: pushes the newly created tag to `origin` automatically.
- In multi-module mode, created tags are prefixed with module `tagPrefix`.
- Release requires a clean Git working tree (no staged/unstaged/untracked changes).
- Git write operations are gated by global `git-write` policy from config.

Suffix handling:

- `--pre <label>` creates/increments pre-release tags for the next core version.
    - Example: `v1.8.0-beta.1`, then `v1.8.0-beta.2`
- `--build <meta>` appends SemVer build metadata.
    - Example: `v1.8.0-beta.2+build.42`
- Baseline bumping uses the latest **stable** tag for the module (pre-release tags do not become baseline
  automatically).

Examples:

```bash
# create a pre-release for next version
chagg release --pre beta

# create a pre-release with build metadata
chagg release --pre preprod --build build.20260314

# compute release version only
chagg release --version-only

# dry-run release
chagg release --dry-run

# create and push tag automatically
chagg release --push
```

## Typical release flow

```bash
# 1) add entries while developing
chagg add api/new-endpoint
chagg add auth/token-fix --type fix --component auth

# 2) validate
chagg check

# 3) preview
chagg log

# 4) generate changelog text
chagg generate > CHANGELOG.md

# 5) create local tag
chagg release

# 6) push tag (command is printed by chagg)
git push origin vX.Y.Z
```
