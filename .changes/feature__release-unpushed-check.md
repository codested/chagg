---
bump: minor
---

The release command now refuses to create a tag when the current branch has commits that have not been pushed to the upstream tracking branch. A tag on an unpushed commit would be unreachable on the remote after pushing the tag alone. The check is silently skipped when no upstream is configured.