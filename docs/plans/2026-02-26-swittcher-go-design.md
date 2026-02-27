# swittcher Go Port Design
_2026-02-26_

## Goal

Port the Rust `switcher` tool to a Go implementation named `swittcher` with:

- cross-platform behavior (Unix + Windows)
- TUI and CLI in v1
- pluggable driver structure (no path/flow hardcoding)

## Architecture

- `cmd/swittcher` - executable wiring
- `internal/cli` - argument parsing + interactive prompt helpers
- `internal/config` - profile/config storage and path resolution
- `internal/driver` - app driver interface + registry
- `internal/driver/codex` - Codex implementation
- `internal/tui` - terminal UI state/actions

## Storage

Base config directory is resolved in this order:

1. `--config-dir`
2. `SWITTCHER_CONFIG_DIR`
3. `os.UserConfigDir()/swittcher`

Profile directories:

`<base>/profiles/<app>/<name>/`

Config file:

`<base>/config.toml`

## Driver contract

Each app driver provides:

- metadata (`ID`, `DisplayName`, `IsAvailable`)
- account lifecycle (`Login`, `Launch`)
- account display data (`ProfileInfo`, `Usage`)

This keeps app-specific logic isolated from the TUI and storage code.

## Runtime flow

- `swittcher` starts TUI loop
- TUI returns actions (`add`, `delete`, `launch`, `quit`)
- main loop applies action via store + driver
- add flow runs login immediately; failed login rolls back profile

CLI shortcut:

- `swittcher add <app> [name]` executes the same add+login flow without TUI

## Cross-platform rules

- config path via `os.UserConfigDir` (OS-native)
- app process launched with isolated home env
  - always set `HOME`
  - on Windows set `USERPROFILE` too
