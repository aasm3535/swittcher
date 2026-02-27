package alias

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	if runtime.GOOS == "windows" {
		return installAliasWindows(aliasName, targetFlag)
	}

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
	_, err = upsertManagedBlockFile(profile, aliasName, shell, block)
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
	case "cmd":
		return fmt.Sprintf("doskey %s=swittcher %s $*", aliasName, targetFlag), nil
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
	case strings.Contains(s, "cmd.exe"):
		return "cmd"
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
	case "cmd":
		return filepath.Join(home, ".swittcher", "cmd", "aliases.cmd"), nil
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
	case "cmd":
		return "Restart CMD session to use alias"
	case "windows":
		return "Restart PowerShell/CMD sessions to use alias"
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
	case "cmd":
		return fmt.Sprintf("echo %s>>\"%s\"", snippet, profile)
	default:
		return block
	}
}

func managedBlock(shell, aliasName, snippet string) string {
	startMarker, endMarker := markersForAlias(aliasName, shell)
	commentPrefix := "#"
	if normalizeShell(shell) == "cmd" {
		commentPrefix = "::"
	}
	return strings.TrimSpace(strings.Join([]string{
		startMarker,
		fmt.Sprintf("%s shell: %s", commentPrefix, normalizeShell(shell)),
		snippet,
		endMarker,
		"",
	}, "\n"))
}

func upsertManagedBlockFile(profilePath, aliasName, shell, block string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return false, err
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	startMarker, endMarker := markersForAlias(aliasName, shell)
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
	case strings.Contains(s, "cmd"):
		return "cmd"
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

func markersForAlias(aliasName, shell string) (string, string) {
	startPrefix := "#"
	endPrefix := "#"
	if normalizeShell(shell) == "cmd" {
		startPrefix = "::"
		endPrefix = "::"
	}
	switch normalizeAliasName(aliasName) {
	case "cc":
		return strings.Replace(startMarkerCC, "#", startPrefix, 1), strings.Replace(endMarkerCC, "#", endPrefix, 1)
	default:
		return strings.Replace(startMarkerCX, "#", startPrefix, 1), strings.Replace(endMarkerCX, "#", endPrefix, 1)
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

func installAliasWindows(aliasName, targetFlag string) (InstallResult, error) {
	psSnippet, err := BuildSnippetFor("powershell", aliasName, targetFlag)
	if err != nil {
		return InstallResult{}, err
	}
	cmdSnippet, err := BuildSnippetFor("cmd", aliasName, targetFlag)
	if err != nil {
		return InstallResult{}, err
	}

	profiles, err := windowsPowerShellProfiles()
	if err != nil {
		return InstallResult{}, err
	}
	psBlock := managedBlock("powershell", aliasName, psSnippet)
	for _, profile := range profiles {
		if _, err := upsertManagedBlockFile(profile, aliasName, "powershell", psBlock); err != nil {
			return InstallResult{
				Shell:   "windows",
				Profile: profile,
				Snippet: psSnippet,
			}, err
		}
	}

	cmdProfile, err := profilePathForShell("cmd")
	if err != nil {
		return InstallResult{}, err
	}
	cmdBlock := managedBlock("cmd", aliasName, cmdSnippet)
	if _, err := upsertManagedBlockFile(cmdProfile, aliasName, "cmd", cmdBlock); err != nil {
		return InstallResult{
			Shell:   "windows",
			Profile: cmdProfile,
			Snippet: cmdSnippet,
		}, err
	}
	if err := ensureCmdAutoRun(cmdProfile); err != nil {
		return InstallResult{
			Shell:   "windows",
			Profile: cmdProfile,
			Snippet: cmdSnippet,
		}, err
	}

	allProfiles := append([]string{}, profiles...)
	allProfiles = append(allProfiles, cmdProfile)

	return InstallResult{
		Shell:      "windows",
		Profile:    strings.Join(allProfiles, "; "),
		Snippet:    psSnippet + "\n" + cmdSnippet,
		Installed:  true,
		SourceHint: sourceHint("windows", ""),
	}, nil
}

func windowsPowerShellProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	paths := []string{
		filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}
	if override := strings.TrimSpace(os.Getenv("POWERSHELL_PROFILE")); override != "" {
		paths = append([]string{override}, paths...)
	}
	return uniqueStrings(paths), nil
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		norm := strings.ToLower(strings.TrimSpace(item))
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, item)
	}
	return out
}

func ensureCmdAutoRun(aliasFile string) error {
	current, err := readCmdAutoRun()
	if err != nil {
		return err
	}
	next := mergeCmdAutoRun(current, aliasFile)
	if strings.TrimSpace(next) == strings.TrimSpace(current) {
		return nil
	}
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Command Processor`, "/v", "AutoRun", "/t", "REG_SZ", "/d", next, "/f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set cmd AutoRun: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func readCmdAutoRun() (string, error) {
	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Command Processor`, "/v", "AutoRun")
	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "unable to find") || strings.Contains(lower, "не удается найти") {
			return "", nil
		}
		return "", fmt.Errorf("query cmd AutoRun: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parseRegQueryValue(string(out)), nil
}

func parseRegQueryValue(raw string) string {
	re := regexp.MustCompile(`REG_[A-Z_]+`)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		m := re.FindStringIndex(line)
		if m == nil {
			continue
		}
		return strings.TrimSpace(line[m[1]:])
	}
	return ""
}

func mergeCmdAutoRun(current, aliasFile string) string {
	snippet := fmt.Sprintf(`if exist "%s" call "%s"`, aliasFile, aliasFile)
	if strings.Contains(strings.ToLower(current), strings.ToLower(aliasFile)) {
		return strings.TrimSpace(current)
	}
	current = strings.TrimSpace(current)
	if current == "" {
		return snippet
	}
	return current + " & " + snippet
}

func IsAliasInstalledForApp(appID string) (bool, error) {
	aliasName, _, err := aliasSpecForApp(appID)
	if err != nil {
		return false, err
	}

	if runtime.GOOS != "windows" {
		shell := DetectShell(runtime.GOOS, os.Getenv("SHELL"))
		profile, err := profilePathForShell(shell)
		if err != nil {
			return false, err
		}
		return profileHasAliasMarker(profile, aliasName, shell)
	}

	profiles, err := windowsPowerShellProfiles()
	if err != nil {
		return false, err
	}
	for _, profile := range profiles {
		ok, err := profileHasAliasMarker(profile, aliasName, "powershell")
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	cmdProfile, err := profilePathForShell("cmd")
	if err != nil {
		return false, err
	}
	cmdMarker, err := profileHasAliasMarker(cmdProfile, aliasName, "cmd")
	if err != nil {
		return false, err
	}
	if !cmdMarker {
		return false, nil
	}
	autoRun, err := readCmdAutoRun()
	if err != nil {
		return false, err
	}
	if !strings.Contains(strings.ToLower(autoRun), strings.ToLower(cmdProfile)) {
		return false, nil
	}
	return true, nil
}

func profileHasAliasMarker(profilePath, aliasName, shell string) (bool, error) {
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	startMarker, endMarker := markersForAlias(aliasName, shell)
	content := string(raw)
	return strings.Contains(content, startMarker) && strings.Contains(content, endMarker), nil
}
