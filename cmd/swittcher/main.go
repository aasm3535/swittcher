package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aasm3535/swittcher/internal/alias"
	"github.com/aasm3535/swittcher/internal/cli"
	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/driver"
	"github.com/aasm3535/swittcher/internal/driver/codex"
	"github.com/aasm3535/swittcher/internal/tui"
)

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

	store, err := config.NewStore(opts.ConfigDir)
	if err != nil {
		return err
	}

	registry := driver.NewRegistry()
	if err := registry.Register(codex.New()); err != nil {
		return err
	}

	switch opts.Command {
	case cli.CommandAdd:
		return runAddCommand(store, registry, opts)
	default:
		return runTUI(store, registry, opts.CodexOnly)
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

	if err := drv.Login(profileDir); err != nil {
		_ = store.RemoveProfile(opts.AddApp, name)
		return err
	}

	_ = syncProfileDetails(store, drv, opts.AddApp, name, "", profileDir)
	fmt.Printf("Account %q added for %s.\n", name, opts.AddApp)
	return nil
}

func runTUI(store *config.Store, registry *driver.Registry, jumpCodex bool) error {
	cfg, err := store.Read()
	if err != nil {
		cfg = config.File{
			AutoSelectLastUsed: true,
			DefaultSlots:       config.DefaultSlotsCount,
		}
	}
	state := initialTUIState(cfg, jumpCodex)

	for {
		action, nextState, err := tui.Run(state, registry.All(), store)
		if err != nil {
			return err
		}
		state = nextState

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
				continue
			}
			slot := action.Slot
			if slot < 1 {
				slot, err = store.NextAvailableSlot(action.AppID)
				if err != nil {
					state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
					continue
				}
			}
			if err := store.AddProfileToSlot(action.AppID, action.ProfileName, slot); err != nil {
				state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
				continue
			}
			profileDir, err := store.ProfileDir(action.AppID, action.ProfileName)
			if err != nil {
				_ = store.RemoveProfile(action.AppID, action.ProfileName)
				state.StatusMessage = fmt.Sprintf("Add failed: %v", err)
				continue
			}
			if err := drv.Login(profileDir); err != nil {
				_ = store.RemoveProfile(action.AppID, action.ProfileName)
				state.StatusMessage = fmt.Sprintf("Login failed: %v", err)
				continue
			}
			if err := syncProfileDetails(store, drv, action.AppID, action.ProfileName, action.Tag, profileDir); err != nil {
				state.StatusMessage = fmt.Sprintf("Account added, metadata sync failed: %v", err)
			} else {
				state.StatusMessage = fmt.Sprintf("Account %q added", action.ProfileName)
			}

			cfgNow, err := store.Read()
			if err == nil && !cfgNow.Alias.CX.Enabled {
				state.ShowAliasPrompt = true
			}
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
			result, err := alias.InstallCX()
			if err != nil {
				_ = store.SetAliasCXStatus(false, result.Shell, err.Error())
				state.ShowAliasPrompt = false
				state.AliasFallbackCommand = alias.ManualInstallCommand(result.Shell, result.Profile)
				if strings.TrimSpace(state.AliasFallbackCommand) == "" {
					state.AliasFallbackCommand = result.Snippet
				}
				state.StatusMessage = fmt.Sprintf("Alias auto-setup failed: %v", err)
				continue
			}
			_ = store.SetAliasCXStatus(true, result.Shell, "")
			state.ShowAliasPrompt = false
			state.AliasFallbackCommand = ""
			state.StatusMessage = "Alias cx configured. " + result.SourceHint
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
	}
}

func initialTUIState(cfg config.File, jumpCodex bool) tui.State {
	if !cfg.OnboardingAccepted {
		return tui.State{Screen: tui.ScreenWelcome}
	}
	if jumpCodex {
		return tui.State{
			Screen:       tui.ScreenAccountSlots,
			CurrentAppID: "codex",
		}
	}
	return tui.State{Screen: tui.ScreenToolPicker}
}

func syncProfileDetails(store *config.Store, drv driver.AppDriver, appID, profileName, tag, profileDir string) error {
	details := config.ProfileDetails{
		Tag: tag,
	}
	info, err := drv.ProfileInfo(profileDir)
	if err == nil {
		details.Email = info.Email
		details.Plan = info.Plan
		details.AccountID = info.AccountID
	}
	if err := store.SetProfileDetails(appID, profileName, details); err != nil {
		return err
	}
	return nil
}
