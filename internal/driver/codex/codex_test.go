package codex

import (
	"encoding/base64"
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
