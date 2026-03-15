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
3. If none exists, keep walking to `.git` (or filesystem root) and use `.changes` there.

For `check`, `chagg` finds the Git root and validates **all** `.changes` directories below it (useful for multi-module
repositories).

## Configuration

`chagg` applies configuration in four layers, each overriding the previous:

1. **Code defaults** — built-in values used when nothing is configured.
2. **User config** — machine-wide settings (`~/.config/chagg/config.yaml` on macOS/Linux, `%AppData%\chagg\config.yaml` on Windows). Override with `CHAGG_USER_CONFIG=/path/to/config.yaml`.
3. **Repo config** — project settings at the Git root (`.chagg.yaml`, `.chagg.yml`, or `chagg.yml`). Only one file is allowed.
4. **Module config** — per-module overrides inside the `modules:` list of the repo config.

All layers share the same schema for `defaults`, `types`, and `git.write`. The `modules` list is only meaningful in the repo config.

### Defaults

The `defaults` section sets fallback values for entry fields. A module inherits defaults from every layer above it, with lower layers taking precedence.

```yaml
defaults:
  audience: public           # string or list
  component: api             # string or list
  rank: 0
```

- `audience`: applied to new entries that omit the `audience:` field. Can be a single string or a list.
- `component`: applied to new entries that omit `component:`. Can be a single string or a list.
- `rank`: default rank for new entries (default `0`).

### Custom change types

The `types` list lets you add new types or override fields of built-in ones. Types are merged across layers: a higher-priority layer can override any field or append entirely new types.

```yaml
types:
  # Add a new type
  - id: breaking
    aliases: [break, bc]
    title: Breaking Changes
    default-bump: major
    order: 0

  # Override a built-in type's default bump
  - id: security
    default-bump: minor
```

Fields:

- `id` (required): unique canonical identifier, used in changelog grouping.
- `aliases`: case-insensitive filename prefixes that map to this type.
- `title`: section heading in generated changelogs. Defaults to `id` capitalised.
- `default-bump`: version bump applied when no `bump:` override is set (`major`, `minor`, or `patch`). Defaults to `patch`.
- `order`: display order within a changelog version group (lower = earlier). Defaults to append after existing types.

Duplicate IDs within a layer, or aliases that conflict with another type's ID or aliases, are rejected with an error.

Built-in types and their defaults:

| ID         | Aliases              | Bump  | Order |
|------------|----------------------|-------|-------|
| `feature`  | `feat`, `enhancement` | minor | 0     |
| `fix`      | `bugfix`, `patch`    | patch | 1     |
| `removal`  | `remove`             | minor | 2     |
| `security` | —                    | patch | 3     |
| `docs`     | `doc`                | patch | 4     |

### Git write policy

Controls which write operations `chagg` may perform. Configurable at user or repo level; repo settings override user settings.

```yaml
git:
  write:
    allow: true          # global kill-switch; false disables all write ops
    operations:
      add-change: true         # stage new change files after `chagg add`
      create-release-tag: true # create local git tags
      push-release-tag: false  # push tags to origin automatically
```

- `git.write.allow`: when `false`, overrides all individual operation flags to disabled.
- Omit the section entirely to inherit the layer above (built-in default: all allowed).

### Multi-module configuration

For monorepos, declare modules in the repo config to set names, tag prefixes, and module-level defaults/types.

```yaml
modules:
  - name: msal-browser
    changes-dir: lib/msal-browser/.changes
    tag-prefix: msal-browser-
    defaults:
      audience: public
    types:
      - id: security
        default-bump: patch

  - name: msal-node
    changes-dir: lib/msal-node/.changes
    tag-prefix: msal-node-
```

Rules:

- `changes-dir` is relative to repo root and is required.
- `tag-prefix` is prepended when tags are read/created (for example `msal-browser-1.4.0`).
- `name` and `tag-prefix` are optional; when omitted they are inferred from the parent directory of `changes-dir`.
- Without a `modules:` list, `chagg` infers modules from discovered `.changes` directories using the parent directory name.
- For a repo-root `.changes`, the inferred `tag-prefix` is empty.
- If two `.changes` directories infer the same module name, commands fail until you add explicit `modules` entries with unique names.

### Full example config

```yaml
# .chagg.yaml (repo config)
defaults:
  audience:
    - public
    - internal
  rank: 0

git:
  write:
    operations:
      push-release-tag: false

types:
  - id: breaking
    aliases: [break]
    title: Breaking Changes
    default-bump: major
    order: 0

modules:
  - name: api
    changes-dir: services/api/.changes
    tag-prefix: api-
    defaults:
      audience: internal
    types:
      - id: breaking
        default-bump: major
```

## Change entry format

Change type is encoded in the filename prefix, not in front matter.

Filename schema (case-insensitive):

- `<type>__<title>.md` (preferred)
- `<type>_<title>.md` (also accepted)

Examples:

- `feature__oauth-login.md`
- `FEAT__quick-fix.md`
- `Feat_small-update.md`

The available type prefixes include the built-in types (`feature`, `fix`, `removal`, `security`, `docs`) plus any custom types defined in config.

Front matter is optional and only needed for overrides.

```markdown
---
component:
  - api
audience: public
bump: major
rank: 80
issue:
  - JIRA-410
release: v2.1.0
---

Add OAuth login support.
```

### Fields

- `bump` (optional): override the default version bump level for this entry (`major`, `minor`, or `patch`). When
  omitted, the bump level is derived from the change type's `default-bump`.
- `component` (optional, string or list)
- `audience` (optional, string or list; defaults to `defaults.audience` when configured)
- `rank` (optional, default `0`; higher numbers are shown first within a type group)
- `issue` (optional, string or list)
- `release` (optional): pins this entry to a specific version
- Additional custom front-matter fields are allowed and ignored by `chagg`.

Notes:

- Default values (omitted `bump`, empty audience, `rank: 0`) are omitted when new files are rendered.
- The body is free Markdown. `log` uses the first non-empty line as preview text.

## Commands

### `chagg add <path>`

Creates a new entry file below `.changes`.

- `chagg add auth/token-expiry --type fix` -> `.changes/auth/fix__token-expiry.md`
- Missing directories are created automatically.
- Supports flags for all entry properties (`--type`, `--bump`, `--component`, `--audience`, `--rank`, `--issue`,
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
- By default in staging view, `log` prints version hints (latest stable + next calculated tag).
- Use `--no-version-hints` to hide those hints.
- In multi-module mode, tags are scoped to the module's `tag-prefix`.
- If invalid change files are present, `log` fails and asks you to run `chagg check`.

Version assignment rules:

1. If `release:` is set, that pinned version is used.
2. Otherwise, the file is assigned by the commit where it was originally added.
3. Edits/moves do not change version assignment.

### `chagg generate`

Generates a changelog grouped by version and change type.

- Default: staging changes + the most recent tagged release (`-n 1 --show-staging`).
- `-n <count>`: number of tagged releases to include, newest first (default `1`, `0` = all).
- `--only-staging`: include only unreleased (staging) changes.
- `--no-show-staging`: omit unreleased (staging) changes.
- `--since <version>`: include that version and all newer, plus staging.
- `--format <markdown|json>`: output format (default `markdown`).
- Filters: `--audience`, `--component`, `--type`.

Constraints:

- `--only-staging` cannot be combined with `--since`.
- `--only-staging` cannot be combined with `-n`.
- `--only-staging` cannot be combined with `--no-show-staging`.

Examples:

```bash
# default: staging + latest release
chagg generate

# all releases + staging
chagg generate -n 0

# staging only
chagg generate --only-staging

# last 3 releases + staging
chagg generate -n 3

# all releases, no staging
chagg generate -n 0 --no-show-staging
```

### `chagg release`

Creates the next release tag from current staging changes.

- If no staging changes exist, nothing is tagged.
- If tags already exist, next SemVer is computed from staging entries using per-entry bump levels:
    - The effective bump level per entry is the `bump` override when set, otherwise the type's `default-bump`.
    - The highest effective bump level across all staging entries is applied.
    - `bump: major` on any entry triggers a major version bump.
- If no SemVer tag exists yet, prompts for the initial version (default `0.1.0`).
- Tag is created **locally only**. `chagg` prints a copy-paste command to push it.
- `--dry-run`: computes and prints what would happen, but does not create/push tags.
- `--version-only`: prints only the computed version (for scripts/export).
- `--push`: pushes the newly created tag to `origin` automatically.
- In multi-module mode, created tags are prefixed with the module `tag-prefix`.
- Release requires a clean Git working tree (no staged/unstaged/untracked changes).
- Git write operations are gated by the `git.write` policy resolved from the config cascade.

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
