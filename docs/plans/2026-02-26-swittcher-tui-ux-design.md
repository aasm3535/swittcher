# swittcher TUI UX Redesign
_2026-02-26_

## Goal

Redesign `swittcher` TUI to be simple, polished, and onboarding-driven across Windows and Unix, while keeping the codebase modular and extensible for future tools (e.g. Claude Code).

## Product Principles

- TUI-first startup: running `swittcher` opens the UI directly.
- Friendly onboarding: clear first-run explanation and safe expectations.
- Minimal friction: account name is optional, sensible defaults everywhere.
- Cross-platform behavior: same flow on Windows and Unix.
- Extensible architecture: no hardcoded app logic in UI core.

## UX Flow

1. Welcome (first-run gate)
   - Shows ASCII `swittcher` logo.
   - Shows short explanation and disclaimer:
     - uses official tool login flows,
     - does not bypass provider account policies,
     - provider restrictions are outside app control.
   - Single action: `Continue`.
   - If app closes before `Continue`, welcome is shown again next launch.

2. Tool picker
   - Items:
     - `Codex` (active)
     - `Claude Code` (coming soon, selectable but shows development message)
   - User picks a tool to proceed.

3. Profile slots (for selected tool)
   - Minimum 3 slots always visible.
   - Filled slots show account profile metadata.
   - Empty slots are actionable: `Add account`.
   - If account count grows, additional slots appear automatically.
   - Last used account is auto-selected by default.

4. Add account wizard
   - Name is optional.
   - Optional tag (e.g. `work`, `personal`, `team`).
   - If name omitted, app auto-generates a profile name.
   - Starts login flow, then updates profile metadata from driver info.

5. Alias setup prompt
   - Prompted after onboarding/add-account completion path.
   - Suggests creating `cx` alias for fast launch via swittcher.
   - Tries automatic shell profile update.
   - If auto setup fails, shows shell-specific command for copy/paste.
   - Supports copy shortcut (`c`) in fallback view.

## Data Model Changes

`config.toml` includes:

- `onboarding_accepted` (bool)
- `auto_select_last_used` (bool, default true)
- `default_slots` (int, default 3)
- alias metadata block for `cx`
- profile list with optional metadata:
  - app/name/id/timestamps
  - email/plan/account_id
  - optional `tag` and `tag_color`
  - `last_used_at`

## Interaction Design

- Bottom hints on each screen for discoverable controls.
- Conservative colors and readable contrast (no gradient-heavy styling).
- Focus-first navigation with arrow keys / `j` `k` / Enter / Esc.
- Contextual status line for errors/success messages.

## Cross-Platform Alias Strategy

- Detect platform/shell and target profile file:
  - Windows PowerShell profile
  - Unix bash/zsh rc file
- Write idempotent managed block:
  - marker start/end lines
  - avoid duplicate insertions
- Fallback when auto-write fails:
  - show exact command snippet for current shell
  - allow copying snippet.

## Non-Goals (This Iteration)

- Full plugin system for third-party tools.
- Complex visual animation or heavy theme customization.
- Deep account analytics beyond currently available driver metadata.
