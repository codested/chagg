---
bump: patch
---

Commands no longer print 'for module ""' in single-module (root) repositories. Adds shared moduleClause() helper that omits the module clause entirely when the module name is empty.