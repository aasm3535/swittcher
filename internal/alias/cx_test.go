package alias

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func TestBuildSnippetBashDefaultCX(t *testing.T) {
	snippet, err := BuildSnippet("bash")
	if err != nil {
		t.Fatalf("build snippet failed: %v", err)
	}
	if !strings.Contains(snippet, "alias cx='swittcher --codex'") {
		t.Fatalf("unexpected bash snippet: %q", snippet)
	}
}

func TestBuildSnippetForCCPowerShell(t *testing.T) {
	snippet, err := BuildSnippetFor("powershell", "cc", "--claude")
	if err != nil {
		t.Fatalf("build snippet failed: %v", err)
	}
	if !strings.Contains(snippet, "function cc { swittcher --claude $args }") {
		t.Fatalf("unexpected powershell snippet: %q", snippet)
	}
}

func TestUpsertManagedBlockIdempotentPerAlias(t *testing.T) {
	current := "# existing\n"
	block := managedBlock("bash", "cx", "alias cx='swittcher --codex'")
	startMarker, endMarker := markersForAlias("cx", "bash")

	next, changed := upsertManagedBlock(current, block, startMarker, endMarker)
	if !changed {
		t.Fatalf("expected first insert to change file")
	}
	again, changed2 := upsertManagedBlock(next, block, startMarker, endMarker)
	if changed2 {
		t.Fatalf("expected second insert to be idempotent")
	}
	if next != again {
		t.Fatalf("expected same content after second insert")
	}
}

func TestUpsertManagedBlockSeparatesCXAndCC(t *testing.T) {
	cxBlock := managedBlock("bash", "cx", "alias cx='swittcher --codex'")
	cxStart, cxEnd := markersForAlias("cx", "bash")
	withCX, _ := upsertManagedBlock("", cxBlock, cxStart, cxEnd)

	ccBlock := managedBlock("bash", "cc", "alias cc='swittcher --claude'")
	ccStart, ccEnd := markersForAlias("cc", "bash")
	withBoth, changed := upsertManagedBlock(withCX, ccBlock, ccStart, ccEnd)
	if !changed {
		t.Fatalf("expected cc insert to change file")
	}
	if !strings.Contains(withBoth, "alias cx='swittcher --codex'") {
		t.Fatalf("expected cx alias to remain")
	}
	if !strings.Contains(withBoth, "alias cc='swittcher --claude'") {
		t.Fatalf("expected cc alias to be added")
	}
}

func TestBuildSnippetForCmd(t *testing.T) {
	snippet, err := BuildSnippetFor("cmd", "cx", "--codex")
	if err != nil {
		t.Fatalf("build snippet failed: %v", err)
	}
	if !strings.Contains(snippet, "doskey cx=swittcher --codex $*") {
		t.Fatalf("unexpected cmd snippet %q", snippet)
	}
}

func TestMergeCmdAutoRun(t *testing.T) {
	aliasFile := `C:\Users\destr\.swittcher\cmd\aliases.cmd`
	next := mergeCmdAutoRun("", aliasFile)
	if !strings.Contains(next, aliasFile) {
		t.Fatalf("expected autorun to contain alias file, got %q", next)
	}
	again := mergeCmdAutoRun(next, aliasFile)
	if again != next {
		t.Fatalf("expected idempotent autorun merge")
	}
}

func TestIsCmdAutoRunNotFoundOutputEnglish(t *testing.T) {
	out := []byte("ERROR: The system was unable to find the specified registry key or value.")
	if !isCmdAutoRunNotFoundOutput(out) {
		t.Fatalf("expected english missing-value output to be recognized")
	}
}

func TestIsCmdAutoRunNotFoundOutputRussianCP866(t *testing.T) {
	msg := "\u041e\u0428\u0418\u0411\u041a\u0410: \u041d\u0435 \u0443\u0434\u0430\u0435\u0442\u0441\u044f \u043d\u0430\u0439\u0442\u0438 \u0443\u043a\u0430\u0437\u0430\u043d\u043d\u044b\u0439 \u0440\u0430\u0437\u0434\u0435\u043b \u0438\u043b\u0438 \u043f\u0430\u0440\u0430\u043c\u0435\u0442\u0440 \u0440\u0435\u0435\u0441\u0442\u0440\u0430."
	out, err := charmap.CodePage866.NewEncoder().Bytes([]byte(msg))
	if err != nil {
		t.Fatalf("encode cp866: %v", err)
	}
	if !isCmdAutoRunNotFoundOutput(out) {
		t.Fatalf("expected cp866 russian missing-value output to be recognized")
	}
}

func TestIsCmdAutoRunNotFoundOutputNegative(t *testing.T) {
	out := []byte("ERROR: Access is denied.")
	if isCmdAutoRunNotFoundOutput(out) {
		t.Fatalf("did not expect access denied to be treated as missing value")
	}
}
