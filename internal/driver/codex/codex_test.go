package codex

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeJWTProfile(t *testing.T) {
	payload := `{"email":"test@example.com","https://api.openai.com/auth":{"chatgpt_plan_type":"pro","chatgpt_account_id":"acc-123"}}`
	token := "h." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".s"

	info, err := decodeJWTProfile(token)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if info.Email != "test@example.com" {
		t.Fatalf("unexpected email %q", info.Email)
	}
	if info.Plan != "pro" {
		t.Fatalf("unexpected plan %q", info.Plan)
	}
	if info.AccountID != "acc-123" {
		t.Fatalf("unexpected account id %q", info.AccountID)
	}
}

func TestUsageReturnsEmptyWhenAccessTokenMissing(t *testing.T) {
	profileDir := t.TempDir()
	writeAuthFile(t, profileDir, authFile{
		Tokens: authTokens{
			AccessToken: "   ",
		},
	})

	stats, err := New().Usage(profileDir)
	if err != nil {
		t.Fatalf("usage failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.WeeklyPct != nil || stats.HourlyPct != nil {
		t.Fatalf("expected empty stats, got %#v", stats)
	}
}

func TestUsageReturnsEmptyOnNon2xx(t *testing.T) {
	var gotAuth string
	var gotAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	prev := usageEndpoint
	usageEndpoint = srv.URL
	t.Cleanup(func() { usageEndpoint = prev })

	profileDir := t.TempDir()
	writeAuthFile(t, profileDir, authFile{
		Tokens: authTokens{
			AccessToken: "token-123",
		},
	})

	stats, err := New().Usage(profileDir)
	if err != nil {
		t.Fatalf("usage failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.WeeklyPct != nil || stats.HourlyPct != nil {
		t.Fatalf("expected empty stats, got %#v", stats)
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("unexpected authorization header %q", gotAuth)
	}
	if gotAgent != "swittcher" {
		t.Fatalf("unexpected user-agent %q", gotAgent)
	}
}

func TestUsageParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limits":{"weekly":{"used_pct":12.5},"hourly":{"used_pct":34.5}}}`))
	}))
	defer srv.Close()

	prev := usageEndpoint
	usageEndpoint = srv.URL
	t.Cleanup(func() { usageEndpoint = prev })

	profileDir := t.TempDir()
	writeAuthFile(t, profileDir, authFile{
		Tokens: authTokens{
			AccessToken: "token-123",
		},
	})

	stats, err := New().Usage(profileDir)
	if err != nil {
		t.Fatalf("usage failed: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.WeeklyPct == nil || *stats.WeeklyPct != 12.5 {
		t.Fatalf("unexpected weekly pct: %#v", stats.WeeklyPct)
	}
	if stats.HourlyPct == nil || *stats.HourlyPct != 34.5 {
		t.Fatalf("unexpected hourly pct: %#v", stats.HourlyPct)
	}
}

func TestUsageReturnsErrorOnInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer srv.Close()

	prev := usageEndpoint
	usageEndpoint = srv.URL
	t.Cleanup(func() { usageEndpoint = prev })

	profileDir := t.TempDir()
	writeAuthFile(t, profileDir, authFile{
		Tokens: authTokens{
			AccessToken: "token-123",
		},
	})

	_, err := New().Usage(profileDir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeAuthFile(t *testing.T, profileDir string, auth authFile) {
	t.Helper()

	raw, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("marshal auth: %v", err)
	}
	authPath := filepath.Join(profileDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(authPath), 0o755); err != nil {
		t.Fatalf("mkdir auth dir: %v", err)
	}
	if err := os.WriteFile(authPath, raw, 0o644); err != nil {
		t.Fatalf("write auth file: %v", err)
	}
}
