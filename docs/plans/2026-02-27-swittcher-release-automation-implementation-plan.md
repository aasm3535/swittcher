# Swittcher Release Automation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automate patch version tags and GitHub releases for every commit to `main`, with tests on Linux/macOS/Windows.

**Architecture:** Use two GitHub Actions workflows: one CI workflow for validation on PR/branch pushes, and one release workflow for main-branch patch bump, tagging, cross-platform build, and release upload. Expose binary version via CLI flag to verify shipped version.

**Tech Stack:** Go 1.25, GitHub Actions, Bash scripting, GitHub Releases API via `gh`.

---

### Task 1: Add CLI Version Flag (TDD)

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `cmd/swittcher/main_test.go`
- Modify: `internal/cli/cli.go`
- Modify: `cmd/swittcher/main.go`

**Step 1: Write the failing tests**
- Add parse test for `--version`.
- Add run test validating `run([]string{"--version"})` returns `nil`.

**Step 2: Run tests to verify failure**

Run: `go test ./internal/cli ./cmd/swittcher`
Expected: FAIL for missing version option handling.

**Step 3: Write minimal implementation**
- Add `ShowVersion` to CLI options.
- Parse `--version` (and `-v`) as version request.
- Print version and return from `run`.
- Add `version` variable in `main` package defaulting to `dev`.

**Step 4: Re-run targeted tests**

Run: `go test ./internal/cli ./cmd/swittcher`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/cli.go internal/cli/cli_test.go cmd/swittcher/main.go cmd/swittcher/main_test.go
git commit -m "feat(cli): add --version flag and runtime version output"
```

### Task 2: Add CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`

**Step 1: Write workflow config**
- Trigger on `pull_request` and non-main `push`.
- Matrix test on Ubuntu, macOS, Windows.
- Setup Go and run `go test ./...`.

**Step 2: Verify syntax and intent**
- Validate YAML by inspection.
- Ensure matrix and Go version are consistent with project.

**Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add cross-platform test workflow"
```

### Task 3: Add Release Workflow with Auto Patch Bump

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Write workflow config**
- Trigger on `push` to `main`.
- Run tests matrix on three OS.
- Compute next `vX.Y.Z` tag (`PATCH+1`).
- Push tag.
- Build binaries for Linux/macOS/Windows targets.
- Package artifacts and publish GitHub Release.

**Step 2: Ensure race-safe behavior**
- Add `concurrency` for main releases.
- Fail if duplicate tag exists.

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci(release): automate patch tags and cross-platform GitHub releases"
```

### Task 4: Add Commit Message Guidance

**Files:**
- Create: `AGENTS.md`

**Step 1: Document commit convention**
- Add concise rule set for commit type, scope, imperative summary, and examples.
- Keep it practical and project-specific.

**Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: add commit message conventions for swittcher"
```

### Task 5: Final Verification

**Files:**
- Verify all modified files.

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 2: Check repository status**

Run: `git status -sb`
Expected: Only intended changes present.

**Step 3: Optional smoke checks**
- `go build ./cmd/swittcher`
- Check `--version` output locally.

**Step 4: Final commit**

```bash
git add .
git commit -m "feat(release): add automated patch releases and CI"
```
