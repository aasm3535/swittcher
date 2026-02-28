package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aasm3535/swittcher/internal/alias"
	"github.com/aasm3535/swittcher/internal/cli"
	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/driver"
	claudedrv "github.com/aasm3535/swittcher/internal/driver/claude"
	"github.com/aasm3535/swittcher/internal/driver/codex"
	geminidrv "github.com/aasm3535/swittcher/internal/driver/gemini"
	"github.com/aasm3535/swittcher/internal/tui"
)

var version = "dev"
var isAliasInstalledForApp = alias.IsAliasInstalledForApp

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, err := cli.Parse(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			fmt.Print(cli.HelpText())
			return nil
		}
		return err
	}
	if opts.ShowVersion {
		fmt.Println(version)
		return nil
	}

	store, err := config.NewStore(opts.ConfigDir)
	if err != nil {
		return err
	}

	registry := driver.NewRegistry()
	if err := registry.Register(codex.New()); err != nil {
		return err
	}
	if err := registry.Register(claudedrv.New()); err != nil {
		return err
	}
	if err := registry.Register(geminidrv.New()); err != nil {
		return err
	}
	jumpAppID := ""
	if opts.CodexOnly {
		jumpAppID = "codex"
	}
	if opts.ClaudeOnly {
		jumpAppID = "claude"
	}
	if opts.GeminiOnly {
		jumpAppID = "gemini"
	}

	switch opts.Command {
	case cli.CommandAdd:
		return runAddCommand(store, registry, opts)
	default:
		return runTUI(store, registry, jumpAppID)
	}
}

func runAddCommand(store *config.Store, registry *driver.Registry, opts cli.Options) error {
	drv, ok := registry.Get(opts.AddApp)
	if !ok {
		return fmt.Errorf("unknown app %q", opts.AddApp)
	}

	name := opts.AddName
	if name == "" {
		var err error
		name, err = cli.PromptProfileName(os.Stdin, os.Stdout, opts.AddApp)
		if err != nil {
			return err
		}
	}

	slot, err := store.NextAvailableSlot(opts.AddApp)
	if err != nil {
		return err
	}
	if err := store.AddProfileToSlot(opts.AddApp, name, slot); err != nil {
		return err
	}

	profileDir, err := store.ProfileDir(opts.AddApp, name)
	if err != nil {
		_ = store.RemoveProfile(opts.AddApp, name)
		return err
	}
	if err := prepareDriverProfile(opts.AddApp, profileDir, "", "", "", "", ""); err != nil {
		_ = store.RemoveProfile(opts.AddApp, name)
		return err
	}

	if err := drv.Login(profileDir); err != nil {
		_ = store.RemoveProfile(opts.AddApp, name)
		return err
	}

	provider := ""
	if opts.AddApp == "claude" {
		provider = claudedrv.ProviderAccount
	}
	_ = syncProfileDetails(store, drv, opts.AddApp, name, "", provider, "", "", profileDir)
	fmt.Printf("Account %q added for %s.\n", name, opts.AddApp)
	return nil
}

func runTUI(store *config.Store, registry *driver.Registry, jumpAppID string) error {
	cfg, err := store.Read()
	if err != nil {
		cfg = config.File{
			AutoSelectLastUsed: true,
			DefaultSlots:       config.DefaultSlotsCount,
		}
	}
	state := initialTUIState(cfg, jumpAppID)

	for {
		action, nextState, err := tui.Run(state, registry.All(), store)
		if err != nil {
			return err
		}
		state = nextState
		prevStatus := state.StatusMessage
		prevStatusTS := state.StatusSetAtUnix

		switch action.Kind {
		case tui.ActionQuit:
			return nil
		case tui.ActionAcceptWelcome:
			if err := store.SetOnboardingAccepted(true); err != nil {
				state.StatusMessage = fmt.Sprintf("Cannot save onboarding: %v", err)
			} else {
				state.StatusMessage = "Welcome complete"
				state.Screen = tui.ScreenToolPicker
			}
		case tui.ActionDelete:
			if err := store.RemoveProfile(action.AppID, action.ProfileName); err != nil {
				state.StatusMessage = fmt.Sprintf("Delete failed: %v", err)
			} else {
				state.StatusMessage = fmt.Sprintf("Deleted %q", action.ProfileName)
			}
		case tui.ActionDeleteSlot:
			if err := store.RemoveSlot(action.AppID, action.Slot); err != nil {
				state.StatusMessage = fmt.Sprintf("Delete failed: %v", err)
			} else {
				state.StatusMessage = fmt.Sprintf("Slot %d deleted", action.Slot)
			}
		case tui.ActionAddSlot:
			slot, err := store.AddSlot(action.AppID)
			if err != nil {
				state.StatusMessage = fmt.Sprintf("Add slot failed: %v", err)
			} else {
				state.StatusMessage = fmt.Sprintf("Added Slot %d", slot)
			}
		case tui.ActionAdd:
			drv, ok := registry.Get(action.AppID)
			if !ok {
				state.StatusMessage = fmt.Sprintf("Unknown app %q", action.AppID)
				break
			}
			slot := action.Slot
			if slot < 1 {
				slot, err = store.NextAvailableSlot(action.AppID)
				if err != nil {
					state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
					break
				}
			}
			if err := store.AddProfileToSlot(action.AppID, action.ProfileName, slot); err != nil {
				state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
				break
			}
			profileDir, err := store.ProfileDir(action.AppID, action.ProfileName)
			if err != nil {
				_ = store.RemoveProfile(action.AppID, action.ProfileName)
				state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
				break
			}
			if err := prepareDriverProfile(
				action.AppID,
				profileDir,
				action.Provider,
				action.APIKey,
				action.BaseURL,
				action.Model,
				action.SmallModel,
			); err != nil {
				_ = store.RemoveProfile(action.AppID, action.ProfileName)
				state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
				break
			}
			if err := drv.Login(profileDir); err != nil {
				_ = store.RemoveProfile(action.AppID, action.ProfileName)
				state.StatusMessage = fmt.Sprintf("Login failed: %v", err)
				break
			}
			if err := syncProfileDetails(
				store,
				drv,
				action.AppID,
				action.ProfileName,
				action.Tag,
				action.Provider,
				action.BaseURL,
				action.Model,
				profileDir,
			); err != nil {
				state.StatusMessage = fmt.Sprintf("Account added, metadata sync failed: %v", err)
			} else {
				state.StatusMessage = fmt.Sprintf("Account %q added", action.ProfileName)
			}

			cfgNow, err := store.Read()
			if err == nil && shouldPromptAliasSetup(cfgNow, action.AppID) {
				state.CurrentAppID = action.AppID
				state.ShowAliasPrompt = true
			}
		case tui.ActionEdit:
			drv, ok := registry.Get(action.AppID)
			if !ok {
				state.StatusMessage = fmt.Sprintf("Unknown app %q", action.AppID)
				break
			}

			newName := strings.TrimSpace(action.ProfileName)
			oldName := strings.TrimSpace(action.ExistingName)
			if oldName == "" {
				oldName = newName
			}
			if newName == "" {
				state.StatusMessage = "Edit failed: profile name cannot be empty"
				break
			}

			if oldName != newName {
				if err := store.RenameProfile(action.AppID, oldName, newName); err != nil {
					state.StatusMessage = fmt.Sprintf("Edit failed: %v", err)
					break
				}
			}

			profileDir, err := store.ProfileDir(action.AppID, newName)
			if err != nil {
				state.StatusMessage = fmt.Sprintf("Edit failed: %v", err)
				break
			}
			if err := prepareDriverProfile(
				action.AppID,
				profileDir,
				action.Provider,
				action.APIKey,
				action.BaseURL,
				action.Model,
				action.SmallModel,
			); err != nil {
				state.StatusMessage = fmt.Sprintf("Edit failed: %v", err)
				break
			}
			if err := syncProfileDetails(
				store,
				drv,
				action.AppID,
				newName,
				action.Tag,
				action.Provider,
				action.BaseURL,
				action.Model,
				profileDir,
			); err != nil {
				state.StatusMessage = fmt.Sprintf("Profile updated, metadata sync failed: %v", err)
				break
			}
			state.StatusMessage = fmt.Sprintf("Profile %q updated", newName)
		case tui.ActionLaunch:
			drv, ok := registry.Get(action.AppID)
			if !ok {
				return fmt.Errorf("unknown app %q", action.AppID)
			}
			profileDir, err := store.ProfileDir(action.AppID, action.ProfileName)
			if err != nil {
				return err
			}
			_ = store.MarkProfileUsed(action.AppID, action.ProfileName)
			return drv.Launch(profileDir)
		case tui.ActionSetupAlias:
			aliasAppID := strings.TrimSpace(state.CurrentAppID)
			if aliasAppID == "" {
				aliasAppID = "codex"
			}
			aliasName, targetFlag := aliasCommandForApp(aliasAppID)
			result, err := alias.InstallForApp(aliasAppID)
			if err != nil {
				_ = setAliasStatusForApp(store, aliasAppID, false, result.Shell, err.Error())
				state.ShowAliasPrompt = false
				state.AliasFallbackCommand = alias.ManualInstallCommandFor(result.Shell, result.Profile, aliasName, targetFlag)
				if strings.TrimSpace(state.AliasFallbackCommand) == "" {
					state.AliasFallbackCommand = result.Snippet
				}
				state.StatusMessage = fmt.Sprintf("Alias auto-setup failed: %v", err)
				break
			}
			_ = setAliasStatusForApp(store, aliasAppID, true, result.Shell, "")
			state.ShowAliasPrompt = false
			state.AliasFallbackCommand = ""
			state.StatusMessage = fmt.Sprintf("Alias %s configured. %s", aliasName, result.SourceHint)
		case tui.ActionSkipAliasSetup:
			state.ShowAliasPrompt = false
			state.StatusMessage = "Alias setup skipped"
		case tui.ActionCopyAliasCommand:
			if err := alias.CopyToClipboard(action.Command); err != nil {
				state.StatusMessage = fmt.Sprintf("Copy failed: %v", err)
			} else {
				state.StatusMessage = "Alias command copied to clipboard"
			}
		case tui.ActionCloseAliasFallback:
			state.AliasFallbackCommand = ""
		default:
		}
		updateStateStatusTimestamp(&state, prevStatus, prevStatusTS, time.Now().UTC())
	}
}

func initialTUIState(cfg config.File, jumpAppID string) tui.State {
	if !cfg.OnboardingAccepted {
		return tui.State{Screen: tui.ScreenWelcome}
	}
	if strings.TrimSpace(jumpAppID) != "" {
		return tui.State{
			Screen:       tui.ScreenAccountSlots,
			CurrentAppID: jumpAppID,
		}
	}
	if appID := aliasPromptAppForConfig(cfg); appID != "" {
		return tui.State{
			Screen:          tui.ScreenAccountSlots,
			CurrentAppID:    appID,
			ShowAliasPrompt: true,
		}
	}
	return tui.State{Screen: tui.ScreenToolPicker}
}

func syncProfileDetails(
	store *config.Store,
	drv driver.AppDriver,
	appID, profileName, tag, provider, baseURL, model, profileDir string,
) error {
	details := config.ProfileDetails{
		Tag:      tag,
		Provider: provider,
		BaseURL:  baseURL,
		Model:    model,
	}
	info, err := drv.ProfileInfo(profileDir)
	if err == nil {
		details.Email = info.Email
		if strings.TrimSpace(details.Model) == "" {
			details.Model = strings.TrimSpace(info.Plan)
		}
		details.Plan = info.Plan
		details.AccountID = info.AccountID
	}
	if err := store.SetProfileDetails(appID, profileName, details); err != nil {
		return err
	}
	return nil
}

func prepareDriverProfile(appID, profileDir, provider, apiKey, baseURL, model, smallModel string) error {
	if appID != "claude" {
		return nil
	}
	return claudedrv.SaveProfileSettings(profileDir, claudedrv.ProfileSettings{
		Provider:   provider,
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		SmallModel: smallModel,
	})
}

func shouldPromptAliasSetup(cfg config.File, appID string) bool {
	// Temporary kill switch: disable auto alias setup/prompt flow.
	return false
}

func setAliasStatusForApp(store *config.Store, appID string, enabled bool, shell, lastError string) error {
	switch strings.ToLower(strings.TrimSpace(appID)) {
	case "claude":
		return store.SetAliasCCStatus(enabled, shell, lastError)
	case "codex":
		return store.SetAliasCXStatus(enabled, shell, lastError)
	default:
		return nil
	}
}

func aliasCommandForApp(appID string) (aliasName, targetFlag string) {
	switch strings.ToLower(strings.TrimSpace(appID)) {
	case "claude":
		return "cc", "--claude"
	default:
		return "cx", "--codex"
	}
}

func aliasPromptAppForConfig(cfg config.File) string {
	hasCodexProfile := false
	hasClaudeProfile := false
	for _, p := range cfg.Profiles {
		switch strings.ToLower(strings.TrimSpace(p.App)) {
		case "codex":
			hasCodexProfile = true
		case "claude":
			hasClaudeProfile = true
		}
	}

	if hasCodexProfile && shouldPromptAliasSetup(cfg, "codex") {
		return "codex"
	}
	if hasClaudeProfile && shouldPromptAliasSetup(cfg, "claude") {
		return "claude"
	}
	return ""
}

func updateStateStatusTimestamp(state *tui.State, previous string, previousTS int64, now time.Time) {
	msg := strings.TrimSpace(state.StatusMessage)
	if msg == "" {
		state.StatusSetAtUnix = 0
		return
	}
	if msg != strings.TrimSpace(previous) || previousTS == 0 {
		state.StatusSetAtUnix = now.Unix()
		return
	}
	state.StatusSetAtUnix = previousTS
}
