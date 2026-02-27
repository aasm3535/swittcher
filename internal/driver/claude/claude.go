package claude

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aasm3535/swittcher/internal/driver"
)

const (
	ProviderAccount     = "account"
	ProviderZAI         = "zai"
	DefaultZAIBaseURL   = "https://api.z.ai/api/anthropic"
	DefaultZAIModel     = "glm-4.6"
	DefaultZAISmallFast = "glm-4.6-flash"
)

type ProfileSettings struct {
	Provider   string `json:"provider"`
	APIKey     string `json:"api_key,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
	Model      string `json:"model,omitempty"`
	SmallModel string `json:"small_model,omitempty"`
}

type Driver struct{}

func New() *Driver {
	return &Driver{}
}

func (d *Driver) ID() string {
	return "claude"
}

func (d *Driver) DisplayName() string {
	return "Claude Code"
}

func (d *Driver) IsAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (d *Driver) Login(profileDir string) error {
	settings, err := LoadProfileSettings(profileDir)
	if err != nil {
		return err
	}
	if settings.Provider == ProviderZAI {
		return nil
	}
	return runClaude(profileDir, settings)
}

func (d *Driver) Launch(profileDir string) error {
	settings, err := LoadProfileSettings(profileDir)
	if err != nil {
		return err
	}
	return runClaude(profileDir, settings)
}

func (d *Driver) ProfileInfo(profileDir string) (driver.ProfileInfo, error) {
	settings, err := LoadProfileSettings(profileDir)
	if err != nil {
		return driver.ProfileInfo{}, err
	}
	if settings.Provider == ProviderZAI {
		return driver.ProfileInfo{
			Email:     "z.ai gateway",
			Plan:      settings.Model,
			AccountID: "zai",
		}, nil
	}
	return driver.ProfileInfo{}, nil
}

func (d *Driver) Usage(profileDir string) (*driver.UsageStats, error) {
	return &driver.UsageStats{}, nil
}

func SaveProfileSettings(profileDir string, in ProfileSettings) error {
	normalized := normalize(in)
	if err := os.MkdirAll(filepath.Dir(settingsPath(profileDir)), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(profileDir), raw, 0o600)
}

func LoadProfileSettings(profileDir string) (ProfileSettings, error) {
	raw, err := os.ReadFile(settingsPath(profileDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return normalize(ProfileSettings{}), nil
		}
		return ProfileSettings{}, err
	}

	var out ProfileSettings
	if err := json.Unmarshal(raw, &out); err != nil {
		return ProfileSettings{}, err
	}
	return normalize(out), nil
}

func settingsPath(profileDir string) string {
	return filepath.Join(profileDir, ".swittcher", "claude.json")
}

func normalize(in ProfileSettings) ProfileSettings {
	in.Provider = strings.TrimSpace(strings.ToLower(in.Provider))
	in.APIKey = strings.TrimSpace(in.APIKey)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.Model = strings.TrimSpace(in.Model)
	in.SmallModel = strings.TrimSpace(in.SmallModel)

	if in.Provider != ProviderZAI {
		in.Provider = ProviderAccount
	}

	if in.Provider == ProviderZAI {
		if in.BaseURL == "" {
			in.BaseURL = DefaultZAIBaseURL
		}
		if in.Model == "" {
			in.Model = DefaultZAIModel
		}
		if in.SmallModel == "" {
			in.SmallModel = DefaultZAISmallFast
		}
	}

	return in
}

func runClaude(profileDir string, settings ProfileSettings) error {
	cmd := exec.Command("claude")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envWithProfileHome(profileDir, settings)
	return cmd.Run()
}

func envWithProfileHome(profileDir string, settings ProfileSettings) []string {
	env := os.Environ()
	env = setEnv(env, "HOME", profileDir)
	if runtime.GOOS == "windows" {
		env = setEnv(env, "USERPROFILE", profileDir)
	}

	for _, key := range []string{
		"ANTHROPIC_AUTH_TOKEN",
		"ANTHROPIC_BASE_URL",
		"ANTHROPIC_MODEL",
		"ANTHROPIC_SMALL_FAST_MODEL",
	} {
		env = unsetEnv(env, key)
	}

	if settings.Provider == ProviderZAI {
		env = setEnv(env, "ANTHROPIC_AUTH_TOKEN", settings.APIKey)
		env = setEnv(env, "ANTHROPIC_BASE_URL", settings.BaseURL)
		env = setEnv(env, "ANTHROPIC_MODEL", settings.Model)
		env = setEnv(env, "ANTHROPIC_SMALL_FAST_MODEL", settings.SmallModel)
	}
	return env
}

func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func unsetEnv(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		out = append(out, item)
	}
	return out
}
