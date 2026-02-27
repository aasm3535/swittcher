package main

import (
	"io"
	"os"
	"testing"

	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/tui"
)

func TestInitialStateShowsWelcomeWhenNotAccepted(t *testing.T) {
	cfg := config.File{
		OnboardingAccepted: false,
	}
	state := initialTUIState(cfg, "")
	if state.Screen != tui.ScreenWelcome {
		t.Fatalf("expected welcome screen, got %q", state.Screen)
	}
}

func TestInitialStateJumpAppAfterOnboarding(t *testing.T) {
	cfg := config.File{
		OnboardingAccepted: true,
	}
	state := initialTUIState(cfg, "codex")
	if state.Screen != tui.ScreenAccountSlots {
		t.Fatalf("expected account slots screen, got %q", state.Screen)
	}
	if state.CurrentAppID != "codex" {
		t.Fatalf("expected current app codex, got %q", state.CurrentAppID)
	}

	state = initialTUIState(cfg, "claude")
	if state.CurrentAppID != "claude" {
		t.Fatalf("expected current app claude, got %q", state.CurrentAppID)
	}
}

func TestInitialStateShowsAliasPromptWhenMissingAliasAndProfileExists(t *testing.T) {
	cfg := config.File{
		OnboardingAccepted: true,
		Profiles: []config.ProfileEntry{
			{App: "codex", Name: "codex-1", Slot: 1},
		},
	}
	state := initialTUIState(cfg, "")
	if state.Screen != tui.ScreenAccountSlots {
		t.Fatalf("expected account slots screen, got %q", state.Screen)
	}
	if state.CurrentAppID != "codex" {
		t.Fatalf("expected current app codex, got %q", state.CurrentAppID)
	}
	if !state.ShowAliasPrompt {
		t.Fatalf("expected alias prompt to be shown")
	}
}

func TestInitialStateSkipsAliasPromptWhenEnabled(t *testing.T) {
	origChecker := isAliasInstalledForApp
	isAliasInstalledForApp = func(appID string) (bool, error) { return true, nil }
	defer func() { isAliasInstalledForApp = origChecker }()

	cfg := config.File{
		OnboardingAccepted: true,
		Profiles: []config.ProfileEntry{
			{App: "codex", Name: "codex-1", Slot: 1},
		},
		Alias: config.AliasConfig{
			CX: config.AliasEntry{Enabled: true},
		},
	}
	state := initialTUIState(cfg, "")
	if state.ShowAliasPrompt {
		t.Fatalf("did not expect alias prompt when cx alias enabled")
	}
	if state.Screen != tui.ScreenToolPicker {
		t.Fatalf("expected tool picker screen, got %q", state.Screen)
	}
}

func TestRunVersionFlag(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := run([]string{"--version"})

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if runErr != nil {
		t.Fatalf("run failed: %v", runErr)
	}
	if len(out) == 0 {
		t.Fatalf("expected version output, got empty")
	}
}

func TestAliasCommandForApp(t *testing.T) {
	aliasName, target := aliasCommandForApp("codex")
	if aliasName != "cx" || target != "--codex" {
		t.Fatalf("unexpected codex alias spec: %q %q", aliasName, target)
	}

	aliasName, target = aliasCommandForApp("claude")
	if aliasName != "cc" || target != "--claude" {
		t.Fatalf("unexpected claude alias spec: %q %q", aliasName, target)
	}
}

func TestShouldPromptAliasSetupPerApp(t *testing.T) {
	origChecker := isAliasInstalledForApp
	isAliasInstalledForApp = func(appID string) (bool, error) { return true, nil }
	defer func() { isAliasInstalledForApp = origChecker }()

	cfg := config.File{}
	if !shouldPromptAliasSetup(cfg, "codex") {
		t.Fatalf("expected codex alias prompt when cx is disabled")
	}
	if !shouldPromptAliasSetup(cfg, "claude") {
		t.Fatalf("expected claude alias prompt when cc is disabled")
	}

	cfg.Alias.CX.Enabled = true
	cfg.Alias.CC.Enabled = true
	if shouldPromptAliasSetup(cfg, "codex") {
		t.Fatalf("did not expect codex alias prompt when cx is enabled")
	}
	if shouldPromptAliasSetup(cfg, "claude") {
		t.Fatalf("did not expect claude alias prompt when cc is enabled")
	}
}

func TestAliasPromptAppForConfig(t *testing.T) {
	origChecker := isAliasInstalledForApp
	isAliasInstalledForApp = func(appID string) (bool, error) { return true, nil }
	defer func() { isAliasInstalledForApp = origChecker }()

	cfg := config.File{
		Profiles: []config.ProfileEntry{
			{App: "claude", Name: "claude-1", Slot: 1},
		},
	}
	if got := aliasPromptAppForConfig(cfg); got != "claude" {
		t.Fatalf("expected claude, got %q", got)
	}

	cfg.Alias.CC.Enabled = true
	if got := aliasPromptAppForConfig(cfg); got != "" {
		t.Fatalf("expected no prompt app, got %q", got)
	}
}

func TestShouldPromptAliasSetupWhenInstalledCheckFails(t *testing.T) {
	origChecker := isAliasInstalledForApp
	isAliasInstalledForApp = func(appID string) (bool, error) { return false, nil }
	defer func() { isAliasInstalledForApp = origChecker }()

	cfg := config.File{
		Alias: config.AliasConfig{
			CX: config.AliasEntry{Enabled: true},
		},
	}
	if !shouldPromptAliasSetup(cfg, "codex") {
		t.Fatalf("expected prompt when enabled flag is stale")
	}
}
