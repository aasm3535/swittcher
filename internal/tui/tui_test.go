package tui

import (
	"strings"
	"testing"

	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/driver"
)

func TestResolveInitialScreenPriority(t *testing.T) {
	cfg := config.File{OnboardingAccepted: false}
	state := State{
		Screen:               ScreenAccountSlots,
		CurrentAppID:         "codex",
		ShowAliasPrompt:      true,
		AliasFallbackCommand: "echo hi",
	}
	if got := resolveInitialScreen(cfg, state); got != ScreenWelcome {
		t.Fatalf("expected welcome to take priority, got %q", got)
	}
}

func TestSidebarWidthClamp(t *testing.T) {
	if got := sidebarWidth(0); got != 32 {
		t.Fatalf("expected fallback width 32, got %d", got)
	}
	if got := sidebarWidth(60); got < 28 || got > 38 {
		t.Fatalf("expected clamped width in [28,38], got %d", got)
	}
	if got := sidebarWidth(180); got != 38 {
		t.Fatalf("expected max clamp 38, got %d", got)
	}
}

func TestStatusLineSymbols(t *testing.T) {
	if got := renderStatusLine("login failed", "fallback"); !strings.Contains(strings.ToLower(got), "login failed") {
		t.Fatalf("expected failure text, got %q", got)
	}
	if got := renderStatusLine("account added", "fallback"); !strings.Contains(strings.ToLower(got), "account added") {
		t.Fatalf("expected success text, got %q", got)
	}
}

type fakeDriver struct {
	id        string
	name      string
	available bool
}

func (d fakeDriver) ID() string { return d.id }

func (d fakeDriver) DisplayName() string { return d.name }

func (d fakeDriver) IsAvailable() bool { return d.available }

func (d fakeDriver) Login(profileDir string) error { return nil }

func (d fakeDriver) Launch(profileDir string) error { return nil }

func (d fakeDriver) ProfileInfo(profileDir string) (driver.ProfileInfo, error) {
	return driver.ProfileInfo{}, nil
}

func (d fakeDriver) Usage(profileDir string) (*driver.UsageStats, error) {
	return &driver.UsageStats{}, nil
}

func TestBuildToolOptionsUsesDriverAvailability(t *testing.T) {
	drivers := []driver.AppDriver{
		fakeDriver{id: "codex", name: "Codex CLI", available: true},
		fakeDriver{id: "claude", name: "Claude Code", available: false},
	}

	opts := buildToolOptions(drivers)
	if len(opts) != 2 {
		t.Fatalf("expected 2 options, got %d", len(opts))
	}
	if !opts[0].Enabled || opts[0].Description != "Ready" {
		t.Fatalf("unexpected first option %+v", opts[0])
	}
	if opts[1].Enabled {
		t.Fatalf("expected second option disabled")
	}
	if !strings.Contains(strings.ToLower(opts[1].Description), "not found in path") {
		t.Fatalf("unexpected disabled description %q", opts[1].Description)
	}
}

func TestAliasPreviewForApp(t *testing.T) {
	aliasName, preview := aliasPreviewForApp("codex")
	if aliasName != "cx" || !strings.Contains(preview, "--codex") {
		t.Fatalf("unexpected codex alias preview: %q %q", aliasName, preview)
	}

	aliasName, preview = aliasPreviewForApp("claude")
	if aliasName != "cc" || !strings.Contains(preview, "--claude") {
		t.Fatalf("unexpected claude alias preview: %q %q", aliasName, preview)
	}
}
