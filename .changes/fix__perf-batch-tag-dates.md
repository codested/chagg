---
bump: patch
---

ListSemVerTags now fetches tag names and dates in a single git call using --format instead of one git log call per tag. This reduces subprocess overhead from O(n tags) to O(1), which is noticeable when there are many release tags.