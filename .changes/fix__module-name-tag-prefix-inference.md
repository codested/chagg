---
type: fix
---

Infer module names and default tag prefixes from the parent directory of each .changes folder (for example
msal-react/.changes → module msal-react with tag prefix msal-react-). Keep the repo-root .changes tag prefix empty, and
fail when inferred module names collide unless explicitly disambiguated in config.
