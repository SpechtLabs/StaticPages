# Contributing to This Project

First off, thank you for considering contributing! We welcome issues, bugfixes, improvements, and new features. This document outlines the standards and process we follow to keep the codebase clean, stable, and maintainable.

---

## Code of Conduct

Please review our [Code of Conduct](./CODE_OF_CONDUCT.md). All contributors are expected to adhere to it.

---

## Getting Started

1. **Fork the repository** and clone it locally. (`gt clone gh:<username>/<reponame> --fork gh:SpechtLabs/<reponame>`)
2. Create a branch using the correct prefix: `fix/` for bugfixes or `feature/` for new features. Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard for determining branch prefixes

---

## Branching and Commit Standards

- Branch names must be descriptive and start with either `fix/` or `feature/`.
  - Example: `fix/login-redirect`, `feature/signup-form`
  - Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard for determining branch prefixes

- Commits must follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification:

  ```plaintext
  <type>[optional scope]: <description>

  [optional body]

  [optional footer(s)]
  ```

  Examples:
  - `fix(auth)!: correct OAuth2 redirect`
  - `feat(cli): add generate-config subcommand`

- Squash your commits into one logical unit before submitting a pull request.

---

## Pull Request Guidelines

Before opening a pull request:

1. Make sure your branch is targeting `main`
2. Your PR title must be descriptive and **must not include emojis**.
3. Your PR description must explain:
   - **What** you changed.
   - **Why** you made the change.
   - Which issue it closes (use `closes #xxxx` syntax).

4. Make sure all checks pass:
   - `mise run check` (for Go code: lint, `go mod tidy`, tests, YAML, workflows, GoReleaser config)
   - `mise run docs-lint` and `mise run docs-build` (for docs-related changes)
   - Unit test coverage **does not decrease**.

---

## Directory-Specific Checks

Every tool is pinned in `.mise.toml`; run `mise install` once, and use the tasks below. `mise tasks` lists them all.

### `/docs` changes

- Run `mise run docs-dev` to validate dev-mode rendering.
- Run `mise run docs-build` to confirm production build passes.

### Go code changes

- Run `mise run fmt` to format Go sources, `go.mod` and Markdown.
- Run `mise run lint` to run golangci-lint with the golint-sl plugin, yamllint and actionlint.
- Run `mise run test` to run all unit tests with the race detector.
- Ensure code coverage is maintained or improved.

---

## Code Style and Tooling

- Use the existing formatting conventions in the repo.
- Do not introduce new dependencies without discussion.
- Avoid committing generated or temporary files.

---

## Contact

If you’re unsure about anything, feel free to file an issue.

Thanks for helping improve this project!
