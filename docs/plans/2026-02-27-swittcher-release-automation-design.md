# Swittcher Release Automation Design

## Goal

Automate versioning and GitHub Releases so every commit to `main` produces a new patch tag and cross-platform binaries, with CI tests across Linux, macOS, and Windows.

## Scope

- Add CI checks for Go tests on all three operating systems.
- Add automated patch-version tag creation on every `push` to `main`.
- Build and attach release artifacts for Linux, macOS, and Windows.
- Expose build version in CLI via `--version`.
- Add repository-level commit message guidelines for contributors and agents.

## Non-Goals

- Manual release workflow.
- Changelog generation from commit history.
- Semver bumping by commit type.

## Architecture

Two workflows:

1. `ci.yml` (validation)
- Triggered on pull requests and non-`main` pushes.
- Runs `go test ./...` on `ubuntu-latest`, `macos-latest`, and `windows-latest`.

2. `release.yml` (shipping)
- Triggered on `push` to `main`.
- Runs tests on all three OS runners.
- Computes next tag from latest `vX.Y.Z` by incrementing `PATCH`.
- Creates and pushes the new git tag.
- Cross-compiles release binaries for selected targets.
- Publishes a GitHub Release and uploads packaged artifacts.

Release workflow uses `concurrency` to serialize main releases and avoid tag collisions between close commits.

## Versioning Rules

- Tag format: `vMAJOR.MINOR.PATCH`.
- If no prior tag exists, first tag is `v0.1.0`.
- On every successful `main` push, bump `PATCH` by 1.

## Build Outputs

Artifacts per release:

- Linux: `amd64`, `arm64`
- macOS: `amd64`, `arm64`
- Windows: `amd64`

Each artifact is packaged as:

- `.tar.gz` for Linux/macOS
- `.zip` for Windows

## Error Handling

- If tests fail in release workflow, no tag/release is created.
- If tag already exists unexpectedly, release workflow fails fast.
- If build or upload fails for any target, workflow fails to avoid partial release artifacts.

## Security and Permissions

- Use built-in `GITHUB_TOKEN` with `contents: write` for tags/releases.
- No external publish tokens required.

## Testing Strategy

- Existing Go unit tests run in both `ci.yml` and release workflow pre-check.
- Add unit coverage for `--version` flag parsing and output path in command entrypoint.

## Operational Notes

- Frequent releases are expected and accepted for this repository.
- Commit message format is documented in `AGENTS.md` for consistency.
