---
bump: patch
---

All file accesses are now bounded to the git repository root. Module changes-dir values in config are validated to be relative and within the root. Symlink targets in .changes directories that escape the root are silently skipped with a warning. Adds gitutil.IsWithinDir helper and tests.