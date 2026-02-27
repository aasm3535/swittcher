# AGENTS.md

## Commit Message Convention

Use Conventional Commits for all commits in this repository.

Format:

`type(scope): short imperative summary`

Examples:

- `feat(cli): add --version flag`
- `fix(config): keep minimum slot count at 4`
- `ci(release): publish cross-platform binaries`
- `docs(readme): clarify config directory override`

## Allowed Types

- `feat` for new behavior
- `fix` for bug fixes
- `ci` for workflows and automation
- `docs` for documentation-only changes
- `refactor` for internal code changes without behavior change
- `test` for tests and test infrastructure
- `chore` for maintenance changes

## Rules

- Keep summary under 72 characters where possible.
- Use imperative mood: `add`, `fix`, `update`, `remove`.
- Keep each commit focused on one logical change.
- Add a body when context is needed (why, constraints, follow-up).

## Release Note

Release versioning is automated in CI:
- every successful `push` to `main` creates the next patch tag (`vX.Y.Z`)
- commit type does not change version bump behavior
