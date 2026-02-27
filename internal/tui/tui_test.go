package tui

import (
	"strings"
	"testing"

	"github.com/aasm3535/swittcher/internal/config"
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
	if got := renderStatusLine("login failed", "fallback"); !strings.Contains(got, "✗ ") {
		t.Fatalf("expected failure symbol, got %q", got)
	}
	if got := renderStatusLine("account added", "fallback"); !strings.Contains(got, "✓ ") {
		t.Fatalf("expected success symbol, got %q", got)
	}
}
