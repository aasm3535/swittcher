package alias

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
)

const (
	startMarkerCX = "# >>> swittcher cx >>>"
	endMarkerCX   = "# <<< swittcher cx <<<"
	startMarkerCC = "# >>> swittcher cc >>>"
	endMarkerCC   = "# <<< swittcher cc <<<"
)

type InstallResult struct {
	Shell      string
	Profile    string
	Snippet    string
	Installed  bool
	SourceHint string
}

func InstallForApp(appID string) (InstallResult, error) {
	aliasName, targetFlag, err := aliasSpecForApp(appID)
	if err != nil {
		return InstallResult{}, err
	}
	return InstallAlias(aliasName, targetFlag)
}

func InstallCX() (InstallResult, error) {
	return InstallAlias("cx", "--codex")
}

func InstallCC() (InstallResult, error) {
	return InstallAlias("cc", "--claude")
}

func InstallAlias(aliasName, targetFlag string) (InstallResult, error) {
	shell := DetectShell(runtime.GOOS, os.Getenv("SHELL"))
	profile, err := profilePathForShell(shell)
	if err != nil {
		return InstallResult{}, err
	}
	snippet, err := BuildSnippetFor(shell, aliasName, targetFlag)
	if err != nil {
		return InstallResult{}, err
	}
	block := managedBlock(shell, aliasName, snippet)
	_, err = upsertManagedBlockFile(profile, aliasName, block)
	if err != nil {
		return InstallResult{
			Shell:   shell,
			Profile: profile,
			Snippet: snippet,
		}, err
	}
	return InstallResult{
		Shell:      shell,
		Profile:    profile,
		Snippet:    snippet,
		Installed:  true,
		SourceHint: sourceHint(shell, profile),
	}, nil
}

func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

func BuildSnippet(shell string) (string, error) {
	return BuildSnippetFor(shell, "cx", "--codex")
}

func BuildSnippetFor(shell, aliasName, targetFlag string) (string, error) {
	aliasName = normalizeAliasName(aliasName)
	targetFlag = strings.TrimSpace(targetFlag)
	if !isValidAliasName(aliasName) {
		return "", fmt.Errorf("invalid alias name %q", aliasName)
	}
	if targetFlag == "" || !strings.HasPrefix(targetFlag, "--") {
		return "", fmt.Errorf("invalid target flag %q", targetFlag)
	}

	switch normalizeShell(shell) {
	case "bash", "zsh":
		return fmt.Sprintf("alias %s='swittcher %s'", aliasName, targetFlag), nil
	case "powershell":
		return fmt.Sprintf("function %s { swittcher %s $args }", aliasName, targetFlag), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func DetectShell(goos, shellEnv string) string {
	s := strings.ToLower(shellEnv)
	switch {
	case strings.Contains(s, "zsh"):
		return "zsh"
	case strings.Contains(s, "bash"):
		return "bash"
	case strings.Contains(s, "pwsh"), strings.Contains(s, "powershell"):
		return "powershell"
	}
	if goos == "windows" {
		return "powershell"
	}
	return "bash"
}

func profilePathForShell(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch normalizeShell(shell) {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "powershell":
		if override := strings.TrimSpace(os.Getenv("POWERSHELL_PROFILE")); override != "" {
			return override, nil
		}
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
}

func sourceHint(shell, profile string) string {
	switch normalizeShell(shell) {
	case "bash", "zsh":
		return fmt.Sprintf("Run: source %s", profile)
	case "powershell":
		return "Restart PowerShell session to use alias"
	default:
		return ""
	}
}

func ManualInstallCommand(shell, profile string) string {
	return ManualInstallCommandFor(shell, profile, "cx", "--codex")
}

func ManualInstallCommandFor(shell, profile, aliasName, targetFlag string) string {
	snippet, err := BuildSnippetFor(shell, aliasName, targetFlag)
	if err != nil {
		return ""
	}
	block := managedBlock(shell, aliasName, snippet)
	switch normalizeShell(shell) {
	case "bash", "zsh":
		return fmt.Sprintf("cat <<'EOF' >> \"%s\"\n%s\nEOF", profile, block)
	case "powershell":
		escaped := strings.ReplaceAll(profile, `"`, "`\"")
		return fmt.Sprintf("$block = @'\n%s\n'@; New-Item -ItemType File -Force -Path \"%s\" | Out-Null; Add-Content -Path \"%s\" -Value \"`n$block`n\"", block, escaped, escaped)
	default:
		return block
	}
}

func managedBlock(shell, aliasName, snippet string) string {
	startMarker, endMarker := markersForAlias(aliasName)
	return strings.TrimSpace(strings.Join([]string{
		startMarker,
		fmt.Sprintf("# shell: %s", normalizeShell(shell)),
		snippet,
		endMarker,
		"",
	}, "\n"))
}

func upsertManagedBlockFile(profilePath, aliasName, block string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return false, err
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	startMarker, endMarker := markersForAlias(aliasName)
	updated, changed := upsertManagedBlock(string(raw), block, startMarker, endMarker)
	if !changed {
		return false, nil
	}
	if err := os.WriteFile(profilePath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func upsertManagedBlock(current, block, startMarker, endMarker string) (string, bool) {
	start := strings.Index(current, startMarker)
	end := strings.Index(current, endMarker)
	if start >= 0 && end >= start {
		end = end + len(endMarker)
		existing := strings.TrimSpace(current[start:end])
		next := strings.TrimSpace(block)
		if existing == next {
			return current, false
		}
		updated := current[:start] + block
		if end < len(current) {
			rest := strings.TrimPrefix(current[end:], "\r\n")
			rest = strings.TrimPrefix(rest, "\n")
			if rest != "" {
				updated += "\n" + rest
			} else {
				updated += "\n"
			}
		} else {
			updated += "\n"
		}
		return updated, true
	}

	trimmed := strings.TrimRight(current, "\r\n")
	if trimmed == "" {
		return block + "\n", true
	}
	return trimmed + "\n\n" + block + "\n", true
}

func normalizeShell(shell string) string {
	s := strings.ToLower(strings.TrimSpace(shell))
	switch {
	case strings.Contains(s, "zsh"):
		return "zsh"
	case strings.Contains(s, "bash"):
		return "bash"
	case strings.Contains(s, "pwsh"), strings.Contains(s, "powershell"):
		return "powershell"
	default:
		return s
	}
}

func normalizeAliasName(aliasName string) string {
	return strings.ToLower(strings.TrimSpace(aliasName))
}

func isValidAliasName(aliasName string) bool {
	if aliasName == "" {
		return false
	}
	for _, r := range aliasName {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func markersForAlias(aliasName string) (string, string) {
	switch normalizeAliasName(aliasName) {
	case "cc":
		return startMarkerCC, endMarkerCC
	default:
		return startMarkerCX, endMarkerCX
	}
}

func aliasSpecForApp(appID string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(appID)) {
	case "codex":
		return "cx", "--codex", nil
	case "claude":
		return "cc", "--claude", nil
	default:
		return "", "", fmt.Errorf("unsupported app id for alias %q", appID)
	}
}
