# swittcher TUI UX Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deliver a first-run-friendly, slot-based, cross-platform TUI with onboarding, optional profile naming, and alias setup for `cx`.

**Architecture:** Keep the current modular Go layout and evolve it with focused packages: `internal/config` for onboarding/settings metadata, `internal/tui` for multi-screen UX, and `internal/alias` for shell integration. Business actions still run through `cmd/swittcher/main.go`, preserving clear boundaries between UI, storage, and drivers.

**Tech Stack:** Go 1.26, `bubbletea`/`lipgloss`/`bubbles` for TUI, TOML config (`BurntSushi/toml`), standard library process/filesystem APIs.

---

### Task 1: Extend Config Schema Defaults and Profile Metadata

**Files:**
- Modify: `internal/config/store.go`
- Test: `internal/config/store_test.go`

**Step 1: Write the failing test**

```go
func TestReadAppliesDefaultsForNewFields(t *testing.T) {
    // Expect onboarding=false, auto_select_last_used=true, default_slots=3
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestReadAppliesDefaultsForNewFields -v`
Expected: FAIL because fields/default logic do not exist yet.

**Step 3: Write minimal implementation**

```go
type File struct {
    OnboardingAccepted bool `toml:"onboarding_accepted"`
    AutoSelectLastUsed bool `toml:"auto_select_last_used"`
    DefaultSlots       int  `toml:"default_slots"`
    Profiles []ProfileEntry `toml:"profiles"`
}
```

Add `applyDefaults` during `Read()`.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestReadAppliesDefaultsForNewFields -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/store.go internal/config/store_test.go
git commit -m "feat(config): add onboarding and slot defaults"
```

### Task 2: Add Last-Used and Profile Metadata Update APIs

**Files:**
- Modify: `internal/config/store.go`
- Test: `internal/config/store_test.go`

**Step 1: Write the failing test**

```go
func TestMarkProfileUsedAndFindLastUsed(t *testing.T) {
    // Create 2 profiles, mark one as used, assert returned last-used name.
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestMarkProfileUsedAndFindLastUsed -v`
Expected: FAIL due missing APIs.

**Step 3: Write minimal implementation**

Add:
- `MarkProfileUsed(appID, profileName string) error`
- `LastUsedProfileName(appID string) (string, bool, error)`
- `SetProfileDetails(...) error`

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config -run TestMarkProfileUsedAndFindLastUsed -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/store.go internal/config/store_test.go
git commit -m "feat(config): track last-used and profile details"
```

### Task 3: Add Alias Integration Package

**Files:**
- Create: `internal/alias/cx.go`
- Create: `internal/alias/cx_test.go`

**Step 1: Write the failing test**

```go
func TestRenderAliasBlockBash(t *testing.T) {
    // Assert block contains "alias cx='swittcher --codex'"
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/alias -run TestRenderAliasBlockBash -v`
Expected: FAIL because package does not exist.

**Step 3: Write minimal implementation**

Implement:
- shell detection
- profile target path resolution
- idempotent managed block render/insert
- fallback command snippet generation

**Step 4: Run test to verify it passes**

Run: `go test ./internal/alias -run TestRenderAliasBlockBash -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/alias/cx.go internal/alias/cx_test.go
git commit -m "feat(alias): add cx alias installer with fallback commands"
```

### Task 4: Redesign TUI Screens and Actions

**Files:**
- Modify: `internal/tui/tui.go`
- Create: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

```go
func TestSlotCountUsesDefaultSlots(t *testing.T) {
    // Profiles=1, defaultSlots=3 => 3 slots
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestSlotCountUsesDefaultSlots -v`
Expected: FAIL because slot helpers do not exist.

**Step 3: Write minimal implementation**

Add:
- welcome screen
- tool picker with disabled Claude item
- slot view helpers for min-slot rendering and empty slot selection
- add wizard form (optional name + optional tag)
- alias fallback modal hooks

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestSlotCountUsesDefaultSlots -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): add welcome, tool picker, and profile slots UX"
```

### Task 5: Wire Main Loop for Onboarding, Last-Used, Alias Prompt

**Files:**
- Modify: `cmd/swittcher/main.go`
- Modify: `internal/config/store.go` (if API extension needed)
- Test: `cmd/swittcher/main_test.go` (or focused helper tests)

**Step 1: Write the failing test**

```go
func TestInitialStateShowsWelcomeWhenNotAccepted(t *testing.T) {
    // Expect welcome screen in returned state.
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/swittcher -run TestInitialStateShowsWelcomeWhenNotAccepted -v`
Expected: FAIL due missing onboarding wiring.

**Step 3: Write minimal implementation**

Wire:
- welcome acceptance updates config
- profile launch marks last-used
- add flow updates details/tag
- alias setup action delegates to `internal/alias`

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/swittcher -run TestInitialStateShowsWelcomeWhenNotAccepted -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/swittcher/main.go cmd/swittcher/main_test.go internal/config/store.go
git commit -m "feat(main): wire onboarding, last-used, and alias flow"
```

### Task 6: Update Documentation and Verify End-to-End

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-02-26-swittcher-tui-ux-design.md`

**Step 1: Write failing behavior check (manual script)**

Create a repeatable checklist script or command list for:
- first-run welcome,
- codex slot add,
- alias prompt behavior,
- fallback snippet visibility.

**Step 2: Run verification command suite**

Run:
- `go test ./...`
- `go build -o swittcher.exe ./cmd/swittcher`
- `./swittcher.exe --help`

Expected: all pass/build/help output valid.

**Step 3: Write minimal doc updates**

Document:
- onboarding flow
- slot behavior
- alias auto/fallback behavior
- keys and shortcuts.

**Step 4: Run verification again**

Run:
- `go test ./...`
- `go build -o swittcher.exe ./cmd/swittcher`

Expected: PASS

**Step 5: Commit**

```bash
git add README.md docs/plans/2026-02-26-swittcher-tui-ux-design.md
git commit -m "docs: describe redesigned swittcher onboarding and slots UX"
```
