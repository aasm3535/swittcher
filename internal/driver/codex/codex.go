package codex

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aasm3535/swittcher/internal/driver"
)

type Driver struct{}

type authFile struct {
	Tokens authTokens `json:"tokens"`
}

type authTokens struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
}

type jwtPayload struct {
	Email      string          `json:"email"`
	OpenAIAuth openAIAuthClaim `json:"https://api.openai.com/auth"`
}

type openAIAuthClaim struct {
	ChatGPTPlanType  string `json:"chatgpt_plan_type"`
	ChatGPTAccountID string `json:"chatgpt_account_id"`
}

var usageEndpoint = "https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27"

func New() *Driver {
	return &Driver{}
}

func (d *Driver) ID() string {
	return "codex"
}

func (d *Driver) DisplayName() string {
	return "Codex CLI"
}

func (d *Driver) IsAvailable() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func (d *Driver) Login(profileDir string) error {
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	if err := copySeedConfig(profileDir); err != nil {
		return err
	}
	return runCodex(profileDir, "login")
}

func (d *Driver) Launch(profileDir string) error {
	return runCodex(profileDir)
}

func (d *Driver) ProfileInfo(profileDir string) (driver.ProfileInfo, error) {
	auth, err := readAuthFile(profileDir)
	if err != nil {
		return driver.ProfileInfo{}, err
	}
	return decodeJWTProfile(auth.Tokens.IDToken)
}

func (d *Driver) Usage(profileDir string) (*driver.UsageStats, error) {
	auth, err := readAuthFile(profileDir)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(auth.Tokens.AccessToken) == "" {
		return &driver.UsageStats{}, nil
	}

	req, err := http.NewRequest(http.MethodGet, usageEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+auth.Tokens.AccessToken)
	req.Header.Set("User-Agent", "swittcher")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &driver.UsageStats{}, nil
	}

	var body struct {
		Limits struct {
			Weekly struct {
				UsedPct *float64 `json:"used_pct"`
			} `json:"weekly"`
			Hourly struct {
				UsedPct *float64 `json:"used_pct"`
			} `json:"hourly"`
		} `json:"limits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return &driver.UsageStats{
		WeeklyPct: body.Limits.Weekly.UsedPct,
		HourlyPct: body.Limits.Hourly.UsedPct,
	}, nil
}

func readAuthFile(profileDir string) (authFile, error) {
	authPath := filepath.Join(profileDir, ".codex", "auth.json")
	raw, err := os.ReadFile(authPath)
	if err != nil {
		return authFile{}, err
	}

	var auth authFile
	if err := json.Unmarshal(raw, &auth); err != nil {
		return authFile{}, err
	}
	return auth, nil
}

func decodeJWTProfile(jwt string) (driver.ProfileInfo, error) {
	parts := strings.SplitN(jwt, ".", 3)
	if len(parts) != 3 {
		return driver.ProfileInfo{}, fmt.Errorf("invalid JWT format")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return driver.ProfileInfo{}, fmt.Errorf("decode JWT payload: %w", err)
	}

	var payload jwtPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return driver.ProfileInfo{}, fmt.Errorf("parse JWT payload: %w", err)
	}

	info := driver.ProfileInfo{
		Email:     "unknown",
		Plan:      "unknown",
		AccountID: payload.OpenAIAuth.ChatGPTAccountID,
	}
	if strings.TrimSpace(payload.Email) != "" {
		info.Email = payload.Email
	}
	if strings.TrimSpace(payload.OpenAIAuth.ChatGPTPlanType) != "" {
		info.Plan = payload.OpenAIAuth.ChatGPTPlanType
	}
	return info, nil
}

func copySeedConfig(profileDir string) error {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	src := filepath.Join(home, ".codex", "config.toml")
	dst := filepath.Join(profileDir, ".codex", "config.toml")

	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func runCodex(profileDir string, args ...string) error {
	cmd := exec.Command("codex", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = envWithProfileHome(profileDir)
	return cmd.Run()
}

func envWithProfileHome(profileDir string) []string {
	env := os.Environ()
	set := func(key, value string) {
		prefix := key + "="
		for i, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[i] = prefix + value
				return
			}
		}
		env = append(env, prefix+value)
	}

	set("HOME", profileDir)
	if runtime.GOOS == "windows" {
		set("USERPROFILE", profileDir)
	}
	return env
}
