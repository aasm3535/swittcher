# Claude Code Dual Auth Design

## Goal

Add Claude Code support to swittcher with two profile-level auth modes:

1. Anthropic account mode
2. Z.AI gateway mode (API key + base URL + custom model names)

Users should be able to keep different modes in different slots for the same app (`claude`).

## Product Requirements

- Claude Code appears as a real, enabled tool when the `claude` binary is available.
- Add-account flow for Claude lets user choose auth mode.
- Z.AI mode supports:
  - API key (stored per profile)
  - Base URL (default `https://api.z.ai/api/anthropic`)
  - Main model
  - Small/fast model
- Slot metadata should show provider/model context.
- Existing Codex behavior remains unchanged.

## Technical Constraints

- Keep current driver interface for minimal blast radius.
- Preserve one app ID (`claude`) so slot switching remains in one screen.
- Avoid storing secrets in global config file.

## Architecture

### Driver Layer

Add new `internal/driver/claude` driver.

- `ID() -> "claude"`
- `DisplayName() -> "Claude Code"`
- `IsAvailable()` checks `claude` in PATH.
- `Launch()` runs `claude` with profile-isolated home and provider-specific env overrides.
- `Login()`:
  - Anthropic account mode: launch `claude` interactively (user can authenticate).
  - Z.AI mode: no interactive login required; profile is ready once env settings are stored.

### Profile Settings Storage

Store Claude provider settings in profile-local JSON:

`<profileDir>/.swittcher/claude.json`

This file stores:
- provider (`account` or `zai`)
- API key (for Z.AI)
- base URL
- model
- small/fast model

Global `config.toml` stores non-secret summary metadata (provider/base/model) for UI display.

### TUI Add Flow

In Claude add form:
- choose mode (toggle account/zai)
- optional profile name + tag (existing behavior)
- if `zai`, request key/base/model/small model

Action payload extends with provider settings to persist per profile.

### Runtime Env Mapping (Z.AI)

On launch for Z.AI profiles, set:
- `ANTHROPIC_AUTH_TOKEN`
- `ANTHROPIC_BASE_URL`
- `ANTHROPIC_MODEL`
- `ANTHROPIC_SMALL_FAST_MODEL`

Based on official gateway docs and Z.AI Claude integration docs.

## Verified External Behavior

From official docs:
- Claude Code supports gateway auth via `ANTHROPIC_AUTH_TOKEN` + `ANTHROPIC_BASE_URL`.
- Z.AI’s Claude-compatible endpoint is `https://api.z.ai/api/anthropic`.
- Supported Z.AI Coding Plan model names are GLM family (for example `glm-4.6`, `glm-4.6-flash`, `glm-4.5`, `glm-4.5-air`), not Claude `sonnet/haiku/opus` names.

## Non-Goals

- Automatic migration of old profiles to Z.AI defaults.
- Full account/usage parsing for Claude account mode.
- In-app validation against remote model catalog.
