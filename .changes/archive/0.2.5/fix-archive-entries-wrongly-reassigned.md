---
type: fix
---

Fix changelog loading to skip the archive/ subdirectory during active entry scanning and instead load archived entries
directly using the archive version directory name. This prevents tidy-archived files from being re-processed through git
history lookup, which caused incorrect version assignments and downstream tidy moves to wrong version folders.
