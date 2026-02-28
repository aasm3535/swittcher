package gemini

import (
	"runtime"
	"strings"
	"testing"
)

func TestDriverMetadata(t *testing.T) {
	d := New()
	if d.ID() != "gemini" {
		t.Fatalf("unexpected id %q", d.ID())
	}
	if d.DisplayName() != "Gemini CLI" {
		t.Fatalf("unexpected display name %q", d.DisplayName())
	}
}

func TestEnvWithProfileHome(t *testing.T) {
	profileDir := "/tmp/swittcher/profile-gemini"
	env := envWithProfileHome(profileDir)
	joined := strings.Join(env, "\n")

	if !strings.Contains(joined, "HOME="+profileDir) {
		t.Fatalf("expected HOME to be set to profile dir")
	}
	if runtime.GOOS == "windows" && !strings.Contains(joined, "USERPROFILE="+profileDir) {
		t.Fatalf("expected USERPROFILE to be set on windows")
	}
}

func TestUsageReturnsEmpty(t *testing.T) {
	stats, err := New().Usage(t.TempDir())
	if err != nil {
		t.Fatalf("usage failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil usage stats")
	}
	if stats.WeeklyPct != nil || stats.HourlyPct != nil {
		t.Fatalf("expected empty usage stats, got %#v", stats)
	}
}
