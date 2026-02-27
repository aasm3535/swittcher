package alias

import (
	"strings"
	"testing"
)

func TestBuildSnippetBash(t *testing.T) {
	snippet, err := BuildSnippet("bash")
	if err != nil {
		t.Fatalf("build snippet failed: %v", err)
	}
	if !strings.Contains(snippet, "alias cx='swittcher --codex'") {
		t.Fatalf("unexpected bash snippet: %q", snippet)
	}
}

func TestBuildSnippetPowerShell(t *testing.T) {
	snippet, err := BuildSnippet("powershell")
	if err != nil {
		t.Fatalf("build snippet failed: %v", err)
	}
	if !strings.Contains(snippet, "function cx") {
		t.Fatalf("expected powershell function snippet, got %q", snippet)
	}
}

func TestUpsertManagedBlockIdempotent(t *testing.T) {
	current := "# existing\n"
	block := managedBlock("bash", "alias cx='swittcher --codex'")

	next, changed := upsertManagedBlock(current, block)
	if !changed {
		t.Fatalf("expected first insert to change file")
	}
	again, changed2 := upsertManagedBlock(next, block)
	if changed2 {
		t.Fatalf("expected second insert to be idempotent")
	}
	if next != again {
		t.Fatalf("expected same content after second insert")
	}
}
