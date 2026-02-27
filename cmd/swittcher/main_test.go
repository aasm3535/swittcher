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
