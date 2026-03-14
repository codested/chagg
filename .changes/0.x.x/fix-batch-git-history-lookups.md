---
type: fix
---

Reduce git history reads during changelog loading by batching file-added date lookups and using per-file fallback only when needed.
