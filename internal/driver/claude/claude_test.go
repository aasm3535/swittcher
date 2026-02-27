package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProfileSettingsDefaults(t *testing.T) {
	tmp := t.TempDir()
	got, err := LoadProfileSettings(tmp)
	if err != nil {
		t.Fatalf("load settings failed: %v", err)
	}
	if got.Provider != ProviderAccount {
		t.Fatalf("expected default provider %q, got %q", ProviderAccount, got.Provider)
	}
}

func TestSaveAndLoadProfileSettings(t *testing.T) {
	tmp := t.TempDir()
	in := ProfileSettings{
		Provider:   ProviderZAI,
		APIKey:     "zai-key-123",
		BaseURL:    "https://api.z.ai/api/anthropic",
		Model:      "glm-4.6",
		SmallModel: "glm-4.6-flash",
	}
	if err := SaveProfileSettings(tmp, in); err != nil {
		t.Fatalf("save settings failed: %v", err)
	}

	path := settingsPath(tmp)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file missing: %v", err)
	}

	got, err := LoadProfileSettings(tmp)
	if err != nil {
		t.Fatalf("load settings failed: %v", err)
	}
	if got.Provider != in.Provider || got.APIKey != in.APIKey {
		t.Fatalf("unexpected loaded settings: %+v", got)
	}
}

func TestEnvWithProfileHomeForZAI(t *testing.T) {
	profileDir := filepath.Join("C:", "tmp", "swittcher", "profile")
	settings := ProfileSettings{
		Provider:   ProviderZAI,
		APIKey:     "secret-token",
		BaseURL:    "https://api.z.ai/api/anthropic",
		Model:      "glm-4.6",
		SmallModel: "glm-4.6-flash",
	}

	env := envWithProfileHome(profileDir, settings)
	joined := strings.Join(env, "\n")

	for _, key := range []string{
		"ANTHROPIC_AUTH_TOKEN=secret-token",
		"ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic",
		"ANTHROPIC_MODEL=glm-4.6",
		"ANTHROPIC_SMALL_FAST_MODEL=glm-4.6-flash",
	} {
		if !strings.Contains(joined, key) {
			t.Fatalf("expected env to contain %q", key)
		}
	}
}
