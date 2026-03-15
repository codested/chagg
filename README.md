# chagg

`chagg` is a release-note workflow for Git repositories.

Instead of writing changelogs at release time, you collect small change entry files in `.changes/`. `chagg` then
validates them, shows release previews, generates Markdown changelogs, and creates the next release tag.

## Quick start

```bash
# one-time setup (creates .changes/ and optional .chagg.yaml)
chagg init

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

For commands that operate on a single working changes directory (`add`, `log`, `generate`, `release`):

1. Start at the current working directory.
2. Walk upward until an existing `.changes` directory is found.
3. If none exists, keep walking to `.git` (or filesystem root) and use `.changes` there.

`add` requires the `.changes` directory to already exist — run `chagg init` first.

For `check`, `chagg` finds the Git root and validates **all** `.changes` directories below it (useful for multi-module repositories).

## Configuration

`chagg` applies configuration in four layers, each overriding the previous:

1. **Code defaults** — built-in values used when nothing is configured.
2. **User config** — machine-wide settings (`~/.config/chagg/config.yaml` on macOS/Linux, `%AppData%\chagg\config.yaml` on Windows). Override with `CHAGG_USER_CONFIG=/path/to/config.yaml`.
3. **Repo config** — project settings at the Git root (`.chagg.yaml`, `.chagg.yml`, or `chagg.yml`). Only one file is allowed.
4. **Module config** — per-module overrides inside the `modules:` list of the repo config.

All layers share the same schema for `defaults`, `types`, and `git.write`. The `modules` list is only meaningful in the repo config.

<details>
<summary><strong>Defaults</strong></summary>

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

</details>

<details>
<summary><strong>Custom change types</strong></summary>

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

| ID         | Aliases               | Bump  | Order |
|------------|-----------------------|-------|-------|
| `feature`  | `feat`, `enhancement` | minor | 0     |
| `fix`      | `bugfix`, `patch`     | patch | 1     |
| `removal`  | `remove`              | minor | 2     |
| `security` | —                     | patch | 3     |
| `docs`     | `doc`                 | patch | 4     |

</details>

<details>
<summary><strong>Git write policy</strong></summary>

Controls which write operations `chagg` may perform. Configurable at user or repo level; repo settings override user settings.

```yaml
git:
  write:
    allow: true          # global kill-switch; false disables all write ops
    operations:
      add-change: true         # stage new change files after `chagg add`
      create-release-tag: true # create local git tags
      push-release-tag: false  # push tags to origin automatically after `chagg release`
```

- `git.write.allow`: when `false`, overrides all individual operation flags to disabled.
- `push-release-tag`: when `true`, `chagg release` pushes the created tag automatically without needing `--push`. Defaults to `false` (local tag only).
- Omit the section entirely to inherit the layer above (built-in default: all operations enabled except `push-release-tag`).

</details>

<details>
<summary><strong>Multi-module configuration</strong></summary>

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

</details>

<details>
<summary><strong>Full example config</strong></summary>

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
      push-release-tag: true   # push tags automatically

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

</details>

## Change entry format

Change type is encoded in the filename prefix, not in front matter.

Filename schema (case-insensitive): `<type>__<title>.md` (preferred) or `<type>_<title>.md`

Examples: `feature__oauth-login.md`, `fix__token-expiry.md`, `docs__release-notes.md`

Front matter is optional and only needed for overrides:

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

<details>
<summary><strong>Front-matter field reference</strong></summary>

- `bump` (optional): override the default version bump level for this entry (`major`, `minor`, or `patch`). When omitted, the bump level is derived from the change type's `default-bump`.
- `component` (optional, string or list)
- `audience` (optional, string or list; defaults to `defaults.audience` when configured)
- `rank` (optional, default `0`; higher numbers are shown first within a type group)
- `issue` (optional, string or list)
- `release` (optional): pins this entry to a specific version
- Additional custom front-matter fields are allowed and ignored by `chagg`.

Notes:

- Default values (omitted `bump`, empty audience, `rank: 0`) are omitted when new files are rendered.
- The body is free Markdown. `log` uses the first non-empty line as preview text.

</details>

## Commands

### `chagg init`

Bootstraps a repository for use with `chagg`.

- Detects the Git root and fails if the current directory is not inside a Git repository.
- If run from a **sub-directory**, prompts whether to create a module for that directory or initialize at the repo root.
- If run from the **repo root**, asks whether this is a multi-module project (default: no).
  - **Single module**: creates `.changes/` at the repo root. No config file is needed.
  - **Multi-module**: prompts for modules (name, changes directory, tag prefix), creates `.changes/` directories, and writes `.chagg.yaml`.
- Warns if bare SemVer tags exist before setting up multi-module mode.
- `--no-prompt`: non-interactive mode; uses all defaults.

### `chagg add <path>`

Creates a new entry file below `.changes`. Requires the `.changes` directory to exist (run `chagg init` first).

- `chagg add auth/token-expiry --type fix` → `.changes/auth/fix__token-expiry.md`
- Supports flags for all entry properties: `--type`, `--bump`, `--component`, `--audience`, `--rank`, `--issue`, `--release`, `--body`.
- New files are staged with `git add` automatically (use `--no-git-add` to skip).
- If the filename already starts with a type prefix, `--type` is optional.
- `--no-prompt` forces non-interactive mode (recommended for CI and AI tooling).

```bash
# AI/automation example
chagg add prototype/ai-note --type docs \
  --body "Generated from PR summaries." --no-prompt
```

### `chagg check`

Validates all change entry files in all discovered `.changes` directories.

- Verifies filename type prefix schema and supported front-matter values.
- Prints deterministic, module-grouped per-file results using repository-relative paths.
- Returns non-zero exit code when invalid entries are found.

### `chagg log [version]`

Shows a human-friendly release preview.

- No argument: shows `staging` (changes since last SemVer tag) with version hints.
- With `version`: shows changes assigned to that specific release.
- Filters: `--audience`, `--component`, `--type`.
- `--preview-length <n>` controls preview truncation (default: `80`).
- `--no-version-hints` hides the latest stable / next tag hints.
- Fails if invalid change files are present (run `chagg check` first).

Version assignment rules:

1. If `release:` is set, that pinned version is used.
2. Otherwise, the file is assigned by the commit where it was originally added.
3. Edits/moves do not change version assignment.

### `chagg generate`

Generates a changelog grouped by version and change type.

- Default: staging changes + the most recent tagged release (`-n 1 --show-staging`).
- `-n <count>`: number of tagged releases to include (default `1`, `0` = all).
- `--only-staging` / `--no-show-staging`: include or exclude staging.
- `--since <version>`: include that version and all newer.
- `--format <markdown|json>`: output format (default `markdown`).
- Filters: `--audience`, `--component`, `--type`.

```bash
chagg generate           # staging + latest release
chagg generate -n 0      # all releases + staging
chagg generate --only-staging
chagg generate -n 0 --no-show-staging
```

### `chagg release`

Creates the next release tag from current staging changes.

- Computes the next SemVer from staging entries: the highest effective bump level wins (`bump:` override, or the type's `default-bump`).
- If no SemVer tag exists yet, prompts for the initial version (default `0.1.0`).
- Tag is created locally. Set `git.write.push-release-tag = true` in config or pass `--push` to push it.
- Requires a clean Git working tree.
- `--dry-run`: compute and print the next version without creating a tag.
- `--version-only`: print only the computed version (for scripts).
- `--push`: push the created tag to `origin`.
- `--pre <label>`: create/increment pre-release tags (e.g. `v1.8.0-beta.1`).
- `--build <meta>`: append SemVer build metadata.

```bash
chagg release             # create local tag
chagg release --push      # create and push tag
chagg release --dry-run   # preview only
chagg release --pre beta  # pre-release
```

### `chagg config`

Inspect and modify chagg settings, similar to `git config`.

```bash
chagg config              # show all resolved settings
chagg config <key>        # read a value
chagg config <key> <val>  # write to repo config
chagg config --global <key> <val>  # write to user config
chagg config --unset <key>
chagg config types        # list available change types
```

Supported keys:

| Key                            | Default | Description                                    |
|--------------------------------|---------|------------------------------------------------|
| `defaults.audience`            | —       | Default audience for new entries               |
| `defaults.rank`                | `0`     | Default rank for new entries                   |
| `defaults.component`           | —       | Default component for new entries              |
| `git.write.allow`              | `true`  | Global kill-switch for all git writes          |
| `git.write.add-change`         | `true`  | Stage new change files after `chagg add`       |
| `git.write.create-release-tag` | `true`  | Create local release tags                      |
| `git.write.push-release-tag`   | `false` | Push tags to origin automatically              |

## FAQ

<details>
<summary>How do I push tags to origin automatically?</summary>

Set `push-release-tag` to `true` — `chagg release` will then push without needing `--push`:

```bash
chagg config git.write.push-release-tag true        # this repo only
chagg config --global git.write.push-release-tag true  # all repos
```

To disable automatic pushing again:

```bash
chagg config git.write.push-release-tag false
```

To disable all git writes entirely:

```bash
chagg config --global git.write.allow false
```

</details>

<details>
<summary>How do I prevent chagg from automatically staging files after <code>add</code>?</summary>

```bash
chagg config --global git.write.add-change false  # all repos
chagg config git.write.add-change false           # this repo only
chagg add auth/fix --no-git-add                   # single invocation
```

</details>

<details>
<summary>How do I set a default audience?</summary>

```bash
chagg config --global defaults.audience public          # all repos
chagg config defaults.audience public                   # this repo
chagg config defaults.audience public internal          # multiple values
```

</details>

<details>
<summary>How do I add a custom change type?</summary>

Edit `.chagg.yaml` directly (custom types cannot be set via `chagg config`):

```yaml
# .chagg.yaml
types:
  - id: breaking
    aliases: [break, bc]
    title: Breaking Changes
    default-bump: major
    order: 0
```

Verify it loaded:

```bash
chagg config types
```

</details>

<details>
<summary>How do I see all resolved settings?</summary>

```bash
chagg config
```

Example output:

```
Defaults:
  defaults.audience             = public
  defaults.rank                 = 0
  defaults.component            =

Git write policy:
  git.write.allow               = true
  git.write.add-change          = true
  git.write.create-release-tag  = true
  git.write.push-release-tag    = false

Types (use 'chagg config types' for details):
  feature
  fix
  removal
  security
  docs
```

</details>

## Typical release flow

```bash
# 0) one-time setup (creates .changes/)
chagg init

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

# 6) push tag (command is printed by chagg, or auto-pushed when configured)
git push origin vX.Y.Z
```
