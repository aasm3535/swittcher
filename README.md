# swittcher

Cross-platform Bubble Tea TUI account switcher for CLI tools.

`swittcher` keeps every account in its own isolated profile home and launches tools under that profile.

## Current support

- Codex CLI (`codex`)
- Claude Code (`claude`)

## What changed in this build

- TUI was rewritten from scratch using `Bubble Tea` + `Lip Gloss`
- Welcome onboarding screen (shown until accepted)
- Tool picker screen
- Fixed split slot screen:
  - ASCII logo block
  - separate left staircase: `Codex`, `Slot 1..N`, `Add slot`
  - right detail pane
- Slot behavior:
  - always at least 4 slots
  - delete account from slot keeps slot empty
  - delete empty slot removes slot (but not below 4)
- Add account wizard with:
  - optional profile name (auto-generated if empty)
  - optional tag (work/personal/etc)
- Last-used profile auto-selection
- Alias prompt for `cx`:
  - tries auto setup by shell/OS
  - fallback manual command with copy shortcut

## Config and profile locations

Default base directory:

- Linux/macOS: `~/.config/swittcher`
- Windows: `%AppData%\swittcher`

Override:

- env: `SWITTCHER_CONFIG_DIR`
- flag: `--config-dir`

Layout:

```text
<base>/config.toml
<base>/profiles/<app>/<profile>/
```

## Build

```sh
go build -o swittcher ./cmd/swittcher
```

## Quality checks

```sh
gofmt -w .
go vet ./...
go test ./...
go test -cover ./...
```

`go test -race ./...` also works when `CGO_ENABLED=1` and a C compiler is available.

## Run

```sh
swittcher
```

Codex shortcut:

```sh
swittcher --codex
```

Version:

```sh
swittcher --version
```

CLI add flow (still available):

```sh
swittcher add codex
swittcher add codex work
swittcher add claude work
```

## TUI keys

- `j` / `Down` - move down
- `k` / `Up` - move up
- `Enter` - select / confirm / launch
- `a` - add account
- `d` - delete account
- `?` - help on profile screen
- `q` / `Esc` - back or quit

Alias fallback view:

- `c` - copy manual command
- `Enter` / `Esc` - close fallback dialog

## Claude modes

Claude profiles support two modes in add flow:

- `account` (regular Anthropic login/session)
- `zai` (API gateway mode)

For `zai` mode, swittcher stores profile-local settings and applies:

- `ANTHROPIC_AUTH_TOKEN`
- `ANTHROPIC_BASE_URL`
- `ANTHROPIC_MODEL`
- `ANTHROPIC_SMALL_FAST_MODEL`

Default Z.AI values:

- Base URL: `https://api.z.ai/api/anthropic`
- Model: `glm-4.6`
- Small/Fast model: `glm-4.6-flash`

## Releases

- Every successful push to `main` creates the next patch tag (`vX.Y.Z`)
- GitHub Release artifacts are generated for Linux, macOS, and Windows
