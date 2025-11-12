# Pull Request Template

Before submitting your pull request, please go through the following checklist to ensure it meets our standards.

---

## Pre-Submission Checklist

1. **No Duplicate PRs**
   Search existing pull requests to avoid duplicates.

2. **Branch Naming**
   - Use a descriptive branch name.
   - Branch names must start with `fix/` or `feature/`.
     Examples: `fix/login-bug`, `feature/user-authentication`
     Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard for determining branch prefixes

3. **Commit Style**
   - Follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) standard:

     ```text
     <type>[optional scope]: <description>

     [optional body]

     [optional footer(s)]
     ```

     Examples:
     - `fix(auth): resolve token expiration issue`
     - `feat(docs): add deployment instructions`

4. **Commit Hygiene**
   - Ensure commits are squashed into a single logical commit.
   - The first line of the commit message should be a short, descriptive title.

---

## Opening the Pull Request

- [ ] The target branch is **not** `master` (usually use `staging` instead).
- [ ] The pull request title is clear and descriptive.
- [ ] The description explains the **what** and **why** behind the changes.
- [ ] Use `closes #XXXX` to auto-close related issues, if applicable.
- [ ] **Do not include emojis** in the pull request title or description.

---

## Directory-Specific Requirements

**If you changed files under `/docs`:**

- [ ] `npm run build` completes successfully.

**If you changed files under `/src`:**

- [ ] `go build ./...` completes without errors.
- [ ] `golangci-lint run` shows no warnings.
- [ ] All unit tests pass.
- [ ] Unit test coverage **does not decrease** compared to the main branch.

---

## Reference

Please review the [CONTRIBUTING.md](../CONTRIBUTING.md) file for full guidelines on contributing.

---

PLEASE REMOVE THIS TEMPLATE BEFORE SUBMITTING
