---
type: fix
---

Fix changelog version attribution for moved change files by resolving each file's original add commit with git --follow. This keeps entries on their historical release even after path moves between directories.
