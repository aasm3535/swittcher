package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddListRemoveProfile(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	store.now = func() time.Time { return time.Unix(100, 0).UTC() }

	if err := store.AddProfile("codex", "work"); err != nil {
		t.Fatalf("add profile failed: %v", err)
	}
	if err := store.AddProfile("codex", "work"); err != nil {
		t.Fatalf("idempotent add failed: %v", err)
	}

	list, err := store.ListProfiles("codex")
	if err != nil {
		t.Fatalf("list profiles failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list))
	}
	if list[0].Added == "" {
		t.Fatalf("expected added timestamp")
	}
	if list[0].Slot != 1 {
		t.Fatalf("expected first profile in slot 1, got %d", list[0].Slot)
	}

	if err := store.RemoveProfile("codex", "work"); err != nil {
		t.Fatalf("remove profile failed: %v", err)
	}
	list, err = store.ListProfiles("codex")
	if err != nil {
		t.Fatalf("list profiles failed: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty profiles after remove, got %d", len(list))
	}
}

func TestProfileDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	dir, err := store.ProfileDir("codex", "work")
	if err != nil {
		t.Fatalf("profile dir failed: %v", err)
	}
	expect := filepath.Join(tmp, "profiles", "codex", "work")
	if dir != expect {
		t.Fatalf("unexpected profile dir %q, expected %q", dir, expect)
	}
}

func TestReadAppliesDefaultsForNewFields(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	cfg, err := store.Read()
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if cfg.OnboardingAccepted {
		t.Fatalf("expected onboarding_accepted default false")
	}
	if !cfg.AutoSelectLastUsed {
		t.Fatalf("expected auto_select_last_used default true")
	}
	if cfg.DefaultSlots != 4 {
		t.Fatalf("expected default_slots=4, got %d", cfg.DefaultSlots)
	}
	if got := cfg.SlotCounts["codex"]; got != 0 {
		t.Fatalf("expected empty slot_counts before app usage, got %d", got)
	}
}

func TestMarkProfileUsedAndFindLastUsed(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	store.now = func() time.Time { return time.Unix(100, 0).UTC() }

	if err := store.AddProfile("codex", "one"); err != nil {
		t.Fatalf("add profile failed: %v", err)
	}
	store.now = func() time.Time { return time.Unix(200, 0).UTC() }
	if err := store.AddProfile("codex", "two"); err != nil {
		t.Fatalf("add profile failed: %v", err)
	}

	if err := store.MarkProfileUsed("codex", "one"); err != nil {
		t.Fatalf("mark profile used failed: %v", err)
	}

	name, ok, err := store.LastUsedProfileName("codex")
	if err != nil {
		t.Fatalf("last used failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true for last used profile")
	}
	if name != "one" {
		t.Fatalf("expected last used profile one, got %q", name)
	}
}

func TestSetProfileDetails(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	if err := store.AddProfile("codex", "one"); err != nil {
		t.Fatalf("add profile failed: %v", err)
	}

	details := ProfileDetails{
		Email:     "one@test.dev",
		Plan:      "pro",
		AccountID: "acc-1",
		Tag:       "work",
		Provider:  "zai",
		BaseURL:   "https://api.z.ai/api/anthropic",
		Model:     "glm-4.6",
	}
	if err := store.SetProfileDetails("codex", "one", details); err != nil {
		t.Fatalf("set profile details failed: %v", err)
	}

	list, err := store.ListProfiles("codex")
	if err != nil {
		t.Fatalf("list profiles failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list))
	}
	if list[0].Email != details.Email || list[0].Plan != details.Plan || list[0].AccountID != details.AccountID {
		t.Fatalf("profile details not persisted")
	}
	if list[0].Tag != details.Tag {
		t.Fatalf("expected tag %q, got %q", details.Tag, list[0].Tag)
	}
	if list[0].Provider != details.Provider {
		t.Fatalf("expected provider %q, got %q", details.Provider, list[0].Provider)
	}
	if list[0].BaseURL != details.BaseURL {
		t.Fatalf("expected base_url %q, got %q", details.BaseURL, list[0].BaseURL)
	}
	if list[0].Model != details.Model {
		t.Fatalf("expected model %q, got %q", details.Model, list[0].Model)
	}
	if list[0].TagColor == "" {
		t.Fatalf("expected computed tag color")
	}
}

func TestSetOnboardingAccepted(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	if err := store.SetOnboardingAccepted(true); err != nil {
		t.Fatalf("set onboarding failed: %v", err)
	}

	cfg, err := store.Read()
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !cfg.OnboardingAccepted {
		t.Fatalf("expected onboarding accepted true")
	}

	raw, err := os.ReadFile(store.ConfigPath())
	if err != nil {
		t.Fatalf("read config file failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected non-empty config file")
	}
}

func TestRenameProfile(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	if err := store.AddProfileToSlot("codex", "work", 2); err != nil {
		t.Fatalf("add profile failed: %v", err)
	}
	oldDir, err := store.ProfileDir("codex", "work")
	if err != nil {
		t.Fatalf("profile dir failed: %v", err)
	}
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("mkdir profile dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write marker failed: %v", err)
	}

	if err := store.RenameProfile("codex", "work", "work-2"); err != nil {
		t.Fatalf("rename profile failed: %v", err)
	}

	list, err := store.ListProfiles("codex")
	if err != nil {
		t.Fatalf("list profiles failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list))
	}
	if list[0].Name != "work-2" {
		t.Fatalf("expected renamed profile work-2, got %q", list[0].Name)
	}
	if list[0].Slot != 2 {
		t.Fatalf("expected slot preserved as 2, got %d", list[0].Slot)
	}

	newDir, err := store.ProfileDir("codex", "work-2")
	if err != nil {
		t.Fatalf("new profile dir failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "marker.txt")); err != nil {
		t.Fatalf("expected marker in renamed dir, stat failed: %v", err)
	}
}

func TestSetAliasCCStatus(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	store.now = func() time.Time { return time.Unix(1234, 0).UTC() }

	if err := store.SetAliasCCStatus(true, "powershell", ""); err != nil {
		t.Fatalf("set alias cc status failed: %v", err)
	}

	cfg, err := store.Read()
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if !cfg.Alias.CC.Enabled {
		t.Fatalf("expected alias cc enabled")
	}
	if cfg.Alias.CC.InstalledAt == "" {
		t.Fatalf("expected alias cc installed_at to be set")
	}
}

func TestReadSanitizesInvalidUTF8InConfig(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	raw := []byte("[alias]\n  [alias.cx]\n    enabled = false\n    last_error = \"bad\x8e\"\n")
	if err := os.WriteFile(store.ConfigPath(), raw, 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	cfg, err := store.Read()
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	if cfg.Alias.CX.LastError != "bad" {
		t.Fatalf("expected sanitized last_error 'bad', got %q", cfg.Alias.CX.LastError)
	}
}

func TestSlotLifecycleRules(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	count, err := store.SlotCount("codex")
	if err != nil {
		t.Fatalf("slot count failed: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected default slot count 4, got %d", count)
	}

	if _, err := store.AddSlot("codex"); err != nil {
		t.Fatalf("add slot failed: %v", err)
	}
	count, _ = store.SlotCount("codex")
	if count != 5 {
		t.Fatalf("expected slot count 5 after add, got %d", count)
	}

	if err := store.AddProfileToSlot("codex", "work", 2); err != nil {
		t.Fatalf("add profile to slot failed: %v", err)
	}
	if err := store.RemoveSlot("codex", 2); err == nil {
		t.Fatalf("expected removing non-empty slot to fail")
	}

	if err := store.RemoveProfile("codex", "work"); err != nil {
		t.Fatalf("remove profile failed: %v", err)
	}
	if err := store.RemoveSlot("codex", 2); err != nil {
		t.Fatalf("remove empty slot failed: %v", err)
	}

	count, _ = store.SlotCount("codex")
	if count != 4 {
		t.Fatalf("expected slot count back to 4, got %d", count)
	}
	if err := store.RemoveSlot("codex", 4); err == nil {
		t.Fatalf("expected remove below minimum slots to fail")
	}
}

func TestNextAvailableSlotUsesGapsBeforeGrowing(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	if err := store.AddProfileToSlot("codex", "one", 1); err != nil {
		t.Fatalf("add profile one failed: %v", err)
	}
	if err := store.AddProfileToSlot("codex", "two", 3); err != nil {
		t.Fatalf("add profile two failed: %v", err)
	}

	slot, err := store.NextAvailableSlot("codex")
	if err != nil {
		t.Fatalf("next slot failed: %v", err)
	}
	if slot != 2 {
		t.Fatalf("expected gap slot 2, got %d", slot)
	}
}
