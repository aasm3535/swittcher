package main

import (
	"testing"

	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/tui"
)

func TestInitialStateShowsWelcomeWhenNotAccepted(t *testing.T) {
	cfg := config.File{
		OnboardingAccepted: false,
	}
	state := initialTUIState(cfg, false)
	if state.Screen != tui.ScreenWelcome {
		t.Fatalf("expected welcome screen, got %q", state.Screen)
	}
}

func TestInitialStateJumpCodexAfterOnboarding(t *testing.T) {
	cfg := config.File{
		OnboardingAccepted: true,
	}
	state := initialTUIState(cfg, true)
	if state.Screen != tui.ScreenAccountSlots {
		t.Fatalf("expected account slots screen, got %q", state.Screen)
	}
	if state.CurrentAppID != "codex" {
		t.Fatalf("expected current app codex, got %q", state.CurrentAppID)
	}
}
