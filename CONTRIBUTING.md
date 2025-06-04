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
   - `npm run build` (for docs-related changes)
   - `go build ./...` and `golangci-lint run` (for Go code)
   - All unit tests pass.
   - Unit test coverage **does not decrease**.

---

## Directory-Specific Checks

### `/docs` changes

- Run `npm run dev` to validate dev-mode rendering.
- Run `npm run build` to confirm production build passes.

### `/src` (Go code) changes

- Run `go build ./...` to verify build success.
- Run `golangci-lint run` to ensure style compliance.
- Run all unit tests (`go test ./...`).
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
