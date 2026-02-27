package config

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	DefaultSlotsCount = 4
)

type ProfileEntry struct {
	ID         string `toml:"id,omitempty"`
	App        string `toml:"app"`
	Name       string `toml:"name"`
	Slot       int    `toml:"slot,omitempty"`
	Added      string `toml:"added"`
	CreatedAt  string `toml:"created_at,omitempty"`
	LastUsedAt string `toml:"last_used_at,omitempty"`
	Email      string `toml:"email,omitempty"`
	Plan       string `toml:"plan,omitempty"`
	AccountID  string `toml:"account_id,omitempty"`
	Tag        string `toml:"tag,omitempty"`
	TagColor   string `toml:"tag_color,omitempty"`
	Provider   string `toml:"provider,omitempty"`
	BaseURL    string `toml:"base_url,omitempty"`
	Model      string `toml:"model,omitempty"`
}

type AliasEntry struct {
	Enabled     bool   `toml:"enabled"`
	Shell       string `toml:"shell,omitempty"`
	InstalledAt string `toml:"installed_at,omitempty"`
	LastError   string `toml:"last_error,omitempty"`
}

type AliasConfig struct {
	CX AliasEntry `toml:"cx"`
	CC AliasEntry `toml:"cc"`
}

type File struct {
	OnboardingAccepted bool           `toml:"onboarding_accepted"`
	AutoSelectLastUsed bool           `toml:"auto_select_last_used"`
	DefaultSlots       int            `toml:"default_slots"`
	SlotCounts         map[string]int `toml:"slot_counts"`
	Alias              AliasConfig    `toml:"alias"`
	Profiles           []ProfileEntry `toml:"profiles"`
}

type ProfileDetails struct {
	Email     string
	Plan      string
	AccountID string
	Tag       string
	Provider  string
	BaseURL   string
	Model     string
}

type Store struct {
	baseDir string
	now     func() time.Time
}

func NewStore(baseDir string) (*Store, error) {
	resolved, err := resolveBaseDir(baseDir)
	if err != nil {
		return nil, err
	}
	return &Store{
		baseDir: resolved,
		now:     time.Now,
	}, nil
}

func (s *Store) BaseDir() string {
	return s.baseDir
}

func (s *Store) ConfigPath() string {
	return filepath.Join(s.baseDir, "config.toml")
}

func (s *Store) ProfileDir(appID, profileName string) (string, error) {
	if err := validateSegment("app id", appID); err != nil {
		return "", err
	}
	if err := validateSegment("profile name", profileName); err != nil {
		return "", err
	}
	return filepath.Join(s.baseDir, "profiles", appID, profileName), nil
}

func (s *Store) Read() (File, error) {
	cfgPath := s.ConfigPath()
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return applyReadDefaults(File{}, ""), nil
		}
		return File{}, err
	}
	cleanRaw := bytes.ToValidUTF8(raw, []byte{})

	var cfg File
	if _, err := toml.Decode(string(cleanRaw), &cfg); err != nil {
		return File{}, err
	}
	return applyReadDefaults(cfg, string(cleanRaw)), nil
}

func (s *Store) Write(cfg File) error {
	cfg = applyWriteDefaults(cfg)

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(s.ConfigPath(), buf.Bytes(), 0o644)
}

func (s *Store) AddProfile(appID, profileName string) error {
	if err := validateSegment("app id", appID); err != nil {
		return err
	}
	if err := validateSegment("profile name", profileName); err != nil {
		return err
	}
	profiles, err := s.ListProfiles(appID)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.Name == profileName {
			return nil
		}
	}

	slot, err := s.NextAvailableSlot(appID)
	if err != nil {
		return err
	}
	return s.AddProfileToSlot(appID, profileName, slot)
}

func (s *Store) AddProfileToSlot(appID, profileName string, slot int) error {
	if err := validateSegment("app id", appID); err != nil {
		return err
	}
	if err := validateSegment("profile name", profileName); err != nil {
		return err
	}
	if slot < 1 {
		return fmt.Errorf("slot must be >= 1")
	}

	cfg, err := s.Read()
	if err != nil {
		return err
	}

	if other, exists := findProfileBySlot(cfg, appID, slot); exists && other.Name != profileName {
		return fmt.Errorf("slot %d is occupied by %q", slot, other.Name)
	}

	ensureAppSlotCount(&cfg, appID)
	if slot > cfg.SlotCounts[appID] {
		cfg.SlotCounts[appID] = slot
	}

	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.App == appID && p.Name == profileName {
			p.Slot = slot
			return s.Write(cfg)
		}
	}

	now := s.now().UTC().Format(time.RFC3339)
	cfg.Profiles = append(cfg.Profiles, ProfileEntry{
		ID:        makeProfileID(appID, profileName, s.now()),
		App:       appID,
		Name:      profileName,
		Slot:      slot,
		Added:     now,
		CreatedAt: now,
	})
	return s.Write(cfg)
}

func (s *Store) RemoveProfile(appID, profileName string) error {
	if err := validateSegment("app id", appID); err != nil {
		return err
	}
	if err := validateSegment("profile name", profileName); err != nil {
		return err
	}

	cfg, err := s.Read()
	if err != nil {
		return err
	}

	filtered := cfg.Profiles[:0]
	for _, p := range cfg.Profiles {
		if p.App == appID && p.Name == profileName {
			continue
		}
		filtered = append(filtered, p)
	}
	cfg.Profiles = filtered

	if err := s.Write(cfg); err != nil {
		return err
	}

	dir, err := s.ProfileDir(appID, profileName)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) ListProfiles(appID string) ([]ProfileEntry, error) {
	if err := validateSegment("app id", appID); err != nil {
		return nil, err
	}
	cfg, err := s.Read()
	if err != nil {
		return nil, err
	}

	out := make([]ProfileEntry, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		if p.App == appID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slot != out[j].Slot {
			return out[i].Slot < out[j].Slot
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *Store) SlotCount(appID string) (int, error) {
	if err := validateSegment("app id", appID); err != nil {
		return 0, err
	}
	cfg, err := s.Read()
	if err != nil {
		return 0, err
	}
	return ensureAppSlotCount(&cfg, appID), nil
}

func (s *Store) AddSlot(appID string) (int, error) {
	if err := validateSegment("app id", appID); err != nil {
		return 0, err
	}
	cfg, err := s.Read()
	if err != nil {
		return 0, err
	}

	count := ensureAppSlotCount(&cfg, appID) + 1
	cfg.SlotCounts[appID] = count
	if err := s.Write(cfg); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) RemoveSlot(appID string, slot int) error {
	if err := validateSegment("app id", appID); err != nil {
		return err
	}
	if slot < 1 {
		return fmt.Errorf("slot must be >= 1")
	}

	cfg, err := s.Read()
	if err != nil {
		return err
	}

	count := ensureAppSlotCount(&cfg, appID)
	if slot > count {
		return fmt.Errorf("slot %d does not exist", slot)
	}
	if count <= cfg.DefaultSlots {
		return fmt.Errorf("cannot remove slot below minimum %d", cfg.DefaultSlots)
	}
	for _, p := range cfg.Profiles {
		if p.App == appID && p.Slot == slot {
			return fmt.Errorf("slot %d is not empty", slot)
		}
	}

	for i := range cfg.Profiles {
		if cfg.Profiles[i].App == appID && cfg.Profiles[i].Slot > slot {
			cfg.Profiles[i].Slot--
		}
	}
	cfg.SlotCounts[appID] = count - 1
	return s.Write(cfg)
}

func (s *Store) NextAvailableSlot(appID string) (int, error) {
	if err := validateSegment("app id", appID); err != nil {
		return 0, err
	}
	cfg, err := s.Read()
	if err != nil {
		return 0, err
	}

	count := ensureAppSlotCount(&cfg, appID)
	used := make(map[int]bool, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		if p.App == appID && p.Slot >= 1 && p.Slot <= count {
			used[p.Slot] = true
		}
	}
	for i := 1; i <= count; i++ {
		if !used[i] {
			return i, nil
		}
	}

	cfg.SlotCounts[appID] = count + 1
	if err := s.Write(cfg); err != nil {
		return 0, err
	}
	return count + 1, nil
}

func (s *Store) NextProfileName(appID string) (string, error) {
	if err := validateSegment("app id", appID); err != nil {
		return "", err
	}
	profiles, err := s.ListProfiles(appID)
	if err != nil {
		return "", err
	}

	exists := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		exists[strings.ToLower(p.Name)] = true
	}
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s-%d", appID, i)
		if !exists[strings.ToLower(candidate)] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot allocate next profile name for %s", appID)
}

func (s *Store) SetProfileDetails(appID, profileName string, details ProfileDetails) error {
	cfg, err := s.Read()
	if err != nil {
		return err
	}

	found := false
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.App != appID || p.Name != profileName {
			continue
		}
		found = true
		p.Email = strings.TrimSpace(details.Email)
		p.Plan = strings.TrimSpace(details.Plan)
		p.AccountID = strings.TrimSpace(details.AccountID)
		p.Tag = strings.TrimSpace(details.Tag)
		p.Provider = strings.TrimSpace(details.Provider)
		p.BaseURL = strings.TrimSpace(details.BaseURL)
		p.Model = strings.TrimSpace(details.Model)
		p.TagColor = ""
		if p.Tag != "" {
			p.TagColor = colorForTag(p.Tag)
		}
		break
	}
	if !found {
		return fmt.Errorf("profile %s/%s not found", appID, profileName)
	}
	return s.Write(cfg)
}

func (s *Store) RenameProfile(appID, oldName, newName string) error {
	if err := validateSegment("app id", appID); err != nil {
		return err
	}
	if err := validateSegment("old profile name", oldName); err != nil {
		return err
	}
	if err := validateSegment("new profile name", newName); err != nil {
		return err
	}
	if oldName == newName {
		return nil
	}

	cfg, err := s.Read()
	if err != nil {
		return err
	}

	foundIdx := -1
	for i := range cfg.Profiles {
		p := cfg.Profiles[i]
		if p.App != appID {
			continue
		}
		if p.Name == newName {
			return fmt.Errorf("profile %s/%s already exists", appID, newName)
		}
		if p.Name == oldName {
			foundIdx = i
		}
	}
	if foundIdx < 0 {
		return fmt.Errorf("profile %s/%s not found", appID, oldName)
	}

	oldDir, err := s.ProfileDir(appID, oldName)
	if err != nil {
		return err
	}
	newDir, err := s.ProfileDir(appID, newName)
	if err != nil {
		return err
	}

	renamedOnDisk := false
	if _, err := os.Stat(oldDir); err == nil {
		if _, err := os.Stat(newDir); err == nil {
			return fmt.Errorf("target profile directory already exists: %s", newDir)
		}
		if err := os.MkdirAll(filepath.Dir(newDir), 0o755); err != nil {
			return err
		}
		if err := os.Rename(oldDir, newDir); err != nil {
			return err
		}
		renamedOnDisk = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	cfg.Profiles[foundIdx].Name = newName
	if err := s.Write(cfg); err != nil {
		if renamedOnDisk {
			_ = os.Rename(newDir, oldDir)
		}
		return err
	}
	return nil
}

func (s *Store) MarkProfileUsed(appID, profileName string) error {
	cfg, err := s.Read()
	if err != nil {
		return err
	}

	found := false
	now := s.now().UTC().Format(time.RFC3339)
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.App == appID && p.Name == profileName {
			p.LastUsedAt = now
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("profile %s/%s not found", appID, profileName)
	}
	return s.Write(cfg)
}

func (s *Store) LastUsedProfileName(appID string) (string, bool, error) {
	profiles, err := s.ListProfiles(appID)
	if err != nil {
		return "", false, err
	}
	bestName := ""
	bestTS := ""
	for _, p := range profiles {
		if strings.TrimSpace(p.LastUsedAt) == "" {
			continue
		}
		if bestTS == "" || p.LastUsedAt > bestTS {
			bestTS = p.LastUsedAt
			bestName = p.Name
		}
	}
	if bestName == "" {
		return "", false, nil
	}
	return bestName, true, nil
}

func (s *Store) SetOnboardingAccepted(accepted bool) error {
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	cfg.OnboardingAccepted = accepted
	return s.Write(cfg)
}

func (s *Store) SetAliasCXStatus(enabled bool, shell, lastError string) error {
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	cfg.Alias.CX.Enabled = enabled
	cfg.Alias.CX.Shell = sanitizeUTF8(strings.TrimSpace(shell))
	cfg.Alias.CX.LastError = sanitizeUTF8(strings.TrimSpace(lastError))
	if enabled {
		cfg.Alias.CX.InstalledAt = s.now().UTC().Format(time.RFC3339)
	}
	return s.Write(cfg)
}

func (s *Store) SetAliasCCStatus(enabled bool, shell, lastError string) error {
	cfg, err := s.Read()
	if err != nil {
		return err
	}
	cfg.Alias.CC.Enabled = enabled
	cfg.Alias.CC.Shell = sanitizeUTF8(strings.TrimSpace(shell))
	cfg.Alias.CC.LastError = sanitizeUTF8(strings.TrimSpace(lastError))
	if enabled {
		cfg.Alias.CC.InstalledAt = s.now().UTC().Format(time.RFC3339)
	}
	return s.Write(cfg)
}

func resolveBaseDir(baseDir string) (string, error) {
	if strings.TrimSpace(baseDir) != "" {
		return filepath.Clean(baseDir), nil
	}
	if env := strings.TrimSpace(os.Getenv("SWITTCHER_CONFIG_DIR")); env != "" {
		return filepath.Clean(env), nil
	}

	cfgRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(cfgRoot, "swittcher"), nil
}

func validateSegment(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", label)
	}
	if strings.ContainsRune(value, os.PathSeparator) || strings.ContainsAny(value, `/\`) {
		return fmt.Errorf("%s cannot contain path separators", label)
	}
	if runtime.GOOS == "windows" && strings.ContainsAny(value, `<>:"|?*`) {
		return fmt.Errorf("%s contains invalid windows path characters", label)
	}
	return nil
}

func applyReadDefaults(cfg File, raw string) File {
	cfg = applyWriteDefaults(cfg)
	if raw == "" || !strings.Contains(raw, "auto_select_last_used") {
		cfg.AutoSelectLastUsed = true
	}
	return cfg
}

func applyWriteDefaults(cfg File) File {
	if cfg.Profiles == nil {
		cfg.Profiles = []ProfileEntry{}
	}
	if cfg.DefaultSlots <= 0 {
		cfg.DefaultSlots = DefaultSlotsCount
	}
	if cfg.SlotCounts == nil {
		cfg.SlotCounts = map[string]int{}
	}
	if cfg.Alias.CX.Shell == "" && runtime.GOOS == "windows" {
		cfg.Alias.CX.Shell = "powershell"
	}
	if cfg.Alias.CC.Shell == "" && runtime.GOOS == "windows" {
		cfg.Alias.CC.Shell = "powershell"
	}
	normalizeAllApps(&cfg)
	return cfg
}

func normalizeAllApps(cfg *File) {
	appSet := make(map[string]bool)
	for app := range cfg.SlotCounts {
		if strings.TrimSpace(app) != "" {
			appSet[app] = true
		}
	}
	for _, p := range cfg.Profiles {
		if strings.TrimSpace(p.App) != "" {
			appSet[p.App] = true
		}
	}
	for appID := range appSet {
		normalizeAppSlots(cfg, appID)
	}
}

func normalizeAppSlots(cfg *File, appID string) {
	count := ensureAppSlotCount(cfg, appID)
	used := map[int]bool{}
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.App != appID {
			continue
		}
		if p.Slot < 1 || p.Slot > count || used[p.Slot] {
			p.Slot = 0
			continue
		}
		used[p.Slot] = true
	}

	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.App != appID || p.Slot != 0 {
			continue
		}
		slot := firstFreeSlot(used, count)
		if slot == 0 {
			count++
			slot = count
		}
		p.Slot = slot
		used[slot] = true
	}
	cfg.SlotCounts[appID] = count
}

func ensureAppSlotCount(cfg *File, appID string) int {
	if cfg.SlotCounts == nil {
		cfg.SlotCounts = map[string]int{}
	}
	count := cfg.SlotCounts[appID]
	if count < cfg.DefaultSlots {
		count = cfg.DefaultSlots
	}
	cfg.SlotCounts[appID] = count
	return count
}

func firstFreeSlot(used map[int]bool, count int) int {
	for i := 1; i <= count; i++ {
		if !used[i] {
			return i
		}
	}
	return 0
}

func findProfileBySlot(cfg File, appID string, slot int) (ProfileEntry, bool) {
	for _, p := range cfg.Profiles {
		if p.App == appID && p.Slot == slot {
			return p, true
		}
	}
	return ProfileEntry{}, false
}

func makeProfileID(appID, profileName string, now time.Time) string {
	return fmt.Sprintf("%s-%s-%d", appID, sanitizeIDToken(profileName), now.UnixNano())
}

func sanitizeIDToken(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	if s == "" {
		return "profile"
	}
	return s
}

func colorForTag(tag string) string {
	palette := []string{
		"#4D7CFE",
		"#0FA67A",
		"#C66B08",
		"#D2447A",
		"#7C58D6",
		"#2F8F9D",
		"#A34D7A",
		"#5E7A17",
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(tag))))
	idx := int(h.Sum32()) % len(palette)
	return palette[idx]
}

func sanitizeUTF8(v string) string {
	return strings.ToValidUTF8(v, "")
}
