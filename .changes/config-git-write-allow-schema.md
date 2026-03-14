---
type: fix
---

Refine config parsing so git-write is global-only and supports the requested allow schema: either allow: true|false or allow with nested per-operation keys (add-change, push-release-tag). Remove config-driven auto-add defaults so add staging behavior defaults are code-defined.
