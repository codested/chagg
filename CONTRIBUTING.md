# Contributing to chagg

Thanks for considering a contribution! Here's how to get started.

## Getting started

1. Fork the repo and clone it locally.
2. Install Go 1.26+ and run `go build ./...` to verify the setup.
3. Create a branch for your change.

## Making changes

- Keep PRs focused — one logical change per PR.
- All new code must have tests. Run `go test ./...` before submitting.
- Format with `gofmt -w .`.
- **Document your change** using the CLI itself:
  ```sh
  go run ./cmd/chagg add <descriptive-slug> --no-prompt --type <type> --bump <bump> --body "<description>"
  ```
  The generated `.changes/` file must be included in your PR.

## Pull requests

- Open a PR against `main`.
- Assign **@bitionaire** if you'd like a review.
- Describe *what* changed and *why*. The change entry covers the user-facing summary; the PR description is for reviewers.

## Reporting bugs & requesting features

Use [GitHub Issues](https://github.com/codested/chagg/issues).

## Code of conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Short version: be respectful, stay on topic, keep it about the code.
