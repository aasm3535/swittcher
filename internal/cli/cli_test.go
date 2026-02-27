package cli

import "testing"

func TestParseCodexFlag(t *testing.T) {
	opts, err := Parse([]string{"--codex"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !opts.CodexOnly {
		t.Fatalf("expected --codex to be true")
	}
	if opts.Command != CommandTUI {
		t.Fatalf("expected tui command, got %q", opts.Command)
	}
}

func TestParseClaudeFlag(t *testing.T) {
	opts, err := Parse([]string{"--claude"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !opts.ClaudeOnly {
		t.Fatalf("expected --claude to be true")
	}
	if opts.Command != CommandTUI {
		t.Fatalf("expected tui command, got %q", opts.Command)
	}
}

func TestParseMutuallyExclusiveJumpFlags(t *testing.T) {
	_, err := Parse([]string{"--codex", "--claude"})
	if err == nil {
		t.Fatalf("expected conflict error for --codex and --claude")
	}
}

func TestParseAddCommand(t *testing.T) {
	opts, err := Parse([]string{"add", "codex", "work"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if opts.Command != CommandAdd {
		t.Fatalf("expected add command, got %q", opts.Command)
	}
	if opts.AddApp != "codex" || opts.AddName != "work" {
		t.Fatalf("unexpected add args: app=%q name=%q", opts.AddApp, opts.AddName)
	}
}

func TestParseAddCommandWithGlobalFlagAfterSubcommand(t *testing.T) {
	opts, err := Parse([]string{"add", "--config-dir", "/tmp/sw", "codex", "work"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if opts.ConfigDir != "/tmp/sw" {
		t.Fatalf("unexpected config dir %q", opts.ConfigDir)
	}
	if opts.AddApp != "codex" || opts.AddName != "work" {
		t.Fatalf("unexpected add args: app=%q name=%q", opts.AddApp, opts.AddName)
	}
}

func TestParseVersionFlag(t *testing.T) {
	opts, err := Parse([]string{"--version"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !opts.ShowVersion {
		t.Fatalf("expected --version to enable ShowVersion")
	}
	if opts.Command != CommandTUI {
		t.Fatalf("expected tui command, got %q", opts.Command)
	}
}
