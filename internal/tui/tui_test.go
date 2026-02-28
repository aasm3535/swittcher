package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/driver"
	tea "github.com/charmbracelet/bubbletea"
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

func TestBuildToolOptionsMarksBetaTools(t *testing.T) {
	drivers := []driver.AppDriver{
		fakeDriver{id: "codex", name: "Codex CLI", available: true},
		fakeDriver{id: "claude", name: "Claude Code", available: true},
		fakeDriver{id: "gemini", name: "Gemini CLI", available: true},
	}

	opts := buildToolOptions(drivers)
	if len(opts) != 3 {
		t.Fatalf("expected 3 options, got %d", len(opts))
	}
	if opts[0].Beta {
		t.Fatalf("did not expect codex to be beta")
	}
	if !opts[1].Beta {
		t.Fatalf("expected claude to be beta")
	}
	if !opts[2].Beta {
		t.Fatalf("expected gemini to be beta")
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

func TestDetectGLMPreset(t *testing.T) {
	if idx := detectGLMPreset("glm-4.6", "glm-4.6-flash"); idx != 0 {
		t.Fatalf("expected balanced preset index 0, got %d", idx)
	}
	if idx := detectGLMPreset("glm-4.7", "glm-4.6-flash"); idx != 1 {
		t.Fatalf("expected coding preset index 1, got %d", idx)
	}
	if idx := detectGLMPreset("custom", "custom"); idx != -1 {
		t.Fatalf("expected custom index -1, got %d", idx)
	}
}

func TestShouldPromptAliasFromConfig(t *testing.T) {
	cfg := config.File{
		Profiles: []config.ProfileEntry{
			{App: "codex", Name: "main", Slot: 1},
		},
	}
	if shouldPromptAliasFromConfig(cfg, "codex") {
		t.Fatalf("did not expect codex alias prompt when auto-setup is disabled")
	}
	cfg.Alias.CX.Enabled = true
	if shouldPromptAliasFromConfig(cfg, "codex") {
		t.Fatalf("did not expect codex alias prompt when auto-setup is disabled")
	}
	if shouldPromptAliasFromConfig(cfg, "claude") {
		t.Fatalf("did not expect claude alias prompt with no profiles")
	}
}

func TestNewModelExpiresStaleStatusMessage(t *testing.T) {
	cfg := config.File{OnboardingAccepted: true}
	state := State{
		Screen:          ScreenToolPicker,
		StatusMessage:   "Deleted profile",
		StatusSetAtUnix: time.Now().Add(-10 * time.Second).Unix(),
	}
	m := newModel(state, cfg, []driver.AppDriver{
		fakeDriver{id: "codex", name: "Codex CLI", available: true},
	}, nil)
	if strings.TrimSpace(m.state.StatusMessage) != "" {
		t.Fatalf("expected stale status message to expire, got %q", m.state.StatusMessage)
	}
}

func TestNewModelKeepsFreshStatusMessage(t *testing.T) {
	cfg := config.File{OnboardingAccepted: true}
	state := State{
		Screen:          ScreenToolPicker,
		StatusMessage:   "Deleted profile",
		StatusSetAtUnix: time.Now().Unix(),
	}
	m := newModel(state, cfg, []driver.AppDriver{
		fakeDriver{id: "codex", name: "Codex CLI", available: true},
	}, nil)
	if strings.TrimSpace(m.state.StatusMessage) == "" {
		t.Fatalf("expected fresh status message to remain visible")
	}
}

func TestWelcomeEnterTransitionsToToolsWithoutQuit(t *testing.T) {
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	m := newModel(
		State{Screen: ScreenWelcome},
		config.File{OnboardingAccepted: false},
		[]driver.AppDriver{fakeDriver{id: "codex", name: "Codex CLI", available: true}},
		store,
	)

	modelAny, _ := m.updateWelcome(tea.KeyMsg{Type: tea.KeyEnter})
	got := modelAny.(*model)
	if got.mode != modeTools {
		t.Fatalf("expected tools mode, got %v", got.mode)
	}
	if got.action.Kind != "" {
		t.Fatalf("did not expect quit action, got %q", got.action.Kind)
	}
}

func TestRenderToolsAlwaysShowsASCIILogo(t *testing.T) {
	cfg := config.File{OnboardingAccepted: true}
	m := newModel(
		State{Screen: ScreenToolPicker},
		cfg,
		[]driver.AppDriver{fakeDriver{id: "codex", name: "Codex CLI", available: true}},
		nil,
	)
	m.width = 44
	m.height = 20

	out := m.renderTools()
	if !strings.Contains(out, "_____") {
		t.Fatalf("expected ascii logo in tool picker, got %q", out)
	}
}

func TestRenderToolsShowsBetaBadgeForBetaTools(t *testing.T) {
	cfg := config.File{OnboardingAccepted: true}
	m := newModel(
		State{Screen: ScreenToolPicker},
		cfg,
		[]driver.AppDriver{
			fakeDriver{id: "codex", name: "Codex CLI", available: true},
			fakeDriver{id: "claude", name: "Claude Code", available: true},
			fakeDriver{id: "gemini", name: "Gemini CLI", available: true},
		},
		nil,
	)
	m.width = 72
	m.height = 20

	out := m.renderTools()
	if !strings.Contains(out, "BETA") {
		t.Fatalf("expected BETA badge in tool picker output, got %q", out)
	}
}

func TestNewModelIgnoresAliasPromptAndFallbackState(t *testing.T) {
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	cfg := config.File{OnboardingAccepted: true}
	m := newModel(
		State{
			Screen:               ScreenAccountSlots,
			CurrentAppID:         "codex",
			ShowAliasPrompt:      true,
			AliasFallbackCommand: "echo stale",
		},
		cfg,
		[]driver.AppDriver{fakeDriver{id: "codex", name: "Codex CLI", available: true}},
		store,
	)

	if m.mode != modeSlots {
		t.Fatalf("expected slots mode, got %v", m.mode)
	}
	if m.state.ShowAliasPrompt {
		t.Fatalf("expected alias prompt state to be cleared")
	}
	if strings.TrimSpace(m.state.AliasFallbackCommand) != "" {
		t.Fatalf("expected alias fallback command to be cleared")
	}
}

func TestSwittcherLogoFirstLineSpacing(t *testing.T) {
	lines := strings.Split(swittcherLogo(), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected logo lines")
	}
	want := " _____       _ _   _      _"
	if lines[0] != want {
		t.Fatalf("unexpected first logo line:\nwant: %q\ngot:  %q", want, lines[0])
	}
}
