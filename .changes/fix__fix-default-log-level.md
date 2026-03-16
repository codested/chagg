---
bump: patch
---

Always install a custom slog handler on startup so the Go runtime's default INFO-level handler is never active. Without --verbose or --debug, only warnings and errors are printed.