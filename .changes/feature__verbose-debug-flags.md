---
bump: patch
---

Replace the single --debug flag with -v/--verbose (INFO-level: key operational steps) and --debug (DEBUG-level: internal details). --debug implies --verbose. This matches common CLI conventions (ansible, curl).