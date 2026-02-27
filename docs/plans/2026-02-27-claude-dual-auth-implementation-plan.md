# Claude Code Dual Auth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support Claude Code profiles with two auth modes (Anthropic account and Z.AI API gateway), selectable per slot.

**Architecture:** Add a dedicated Claude driver with profile-local provider settings, extend TUI add flow to collect mode-specific fields, persist non-secret metadata in config, and launch Claude with mode-specific environment variables.

**Tech Stack:** Go 1.25, Bubble Tea TUI, TOML config, JSON profile sidecar.

---

### Task 1: Add Profile Metadata Fields (TDD)

**Files:**
- Modify: `internal/config/store_test.go`
- Modify: `internal/config/store.go`

**Step 1: Write failing test**
- Extend `TestSetProfileDetails` to assert `Provider`, `BaseURL`, `Model`.

**Step 2: Run targeted test to verify failure**

Run: `go test ./internal/config -run TestSetProfileDetails`
Expected: FAIL on missing fields.

**Step 3: Minimal implementation**
- Add metadata fields to `ProfileEntry` and `ProfileDetails`.
- Persist in `SetProfileDetails`.

**Step 4: Re-run targeted test**

Run: `go test ./internal/config -run TestSetProfileDetails`
Expected: PASS.

### Task 2: Implement Claude Driver

**Files:**
- Create: `internal/driver/claude/claude.go`
- Create: `internal/driver/claude/claude_test.go`

**Step 1: Write failing tests**
- Test save/load of profile settings defaults.
- Test env mapping for Z.AI provider.

**Step 2: Verify tests fail**

Run: `go test ./internal/driver/claude`
Expected: FAIL before implementation.

**Step 3: Minimal implementation**
- Add driver methods (`ID`, `DisplayName`, `IsAvailable`, `Login`, `Launch`, `ProfileInfo`, `Usage`).
- Add profile settings JSON read/write helpers.
- Add env override logic for account/zai modes.

**Step 4: Re-run package tests**

Run: `go test ./internal/driver/claude`
Expected: PASS.

### Task 3: Wire Driver into App Runtime

**Files:**
- Modify: `cmd/swittcher/main.go`
- Modify: `cmd/swittcher/main_test.go` (only if needed)

**Step 1: Add failing behavioral check if needed**
- Ensure registry includes Claude driver.

**Step 2: Implement**
- Register Claude driver.
- In add flow, persist claude profile settings before login.
- Extend sync metadata to save provider/base/model.

**Step 3: Verify compile/tests**

Run: `go test ./cmd/swittcher`
Expected: PASS.

### Task 4: Extend TUI Add Form for Claude Modes

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Add failing tests**
- Add at least one test for tool resolution and/or add-form mode behavior helper.

**Step 2: Implement**
- Build tool list from registered drivers (remove hardcoded "in development" entry).
- Add Claude add-form fields:
  - mode toggle (`account` / `zai`)
  - api key, base url, model, small model (for `zai` mode)
- Extend `Action` payload for provider settings.
- Show provider/model metadata in slot details.

**Step 3: Verify package tests**

Run: `go test ./internal/tui`
Expected: PASS.

### Task 5: End-to-End Verification

**Files:**
- Verify all modified files.

**Step 1: Run full tests**

Run: `go test ./...`
Expected: PASS.

**Step 2: Check git status**

Run: `git status -sb`
Expected: only intended files changed.

**Step 3: Optional local smoke**
- Build binary and run add flow manually in local terminal.
