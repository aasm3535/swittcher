package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/aasm3535/swittcher/internal/config"
	"github.com/aasm3535/swittcher/internal/driver"
	claudedrv "github.com/aasm3535/swittcher/internal/driver/claude"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Screen string

const (
	ScreenWelcome      Screen = "welcome"
	ScreenToolPicker   Screen = "tool_picker"
	ScreenAccountSlots Screen = "account_slots"
)

type uiMode int

const (
	modeWelcome uiMode = iota
	modeTools
	modeSlots
	modeAdd
	modeDeleteConfirm
	modeHelp
	modeAliasPrompt
	modeAliasFallback
)

type deleteTarget int

const (
	deleteNone deleteTarget = iota
	deleteProfile
	deleteSlot
)

type State struct {
	Screen               Screen
	CurrentAppID         string
	Selected             int
	StatusMessage        string
	StatusSetAtUnix      int64
	ShowAliasPrompt      bool
	AliasFallbackCommand string
}

const (
	statusTTL          = 6 * time.Second
	statusTickInterval = 1 * time.Second
)

type statusTickMsg struct{}

type ActionKind string

const (
	ActionNone               ActionKind = "none"
	ActionQuit               ActionKind = "quit"
	ActionAcceptWelcome      ActionKind = "accept_welcome"
	ActionAdd                ActionKind = "add"
	ActionEdit               ActionKind = "edit"
	ActionAddSlot            ActionKind = "add_slot"
	ActionDelete             ActionKind = "delete"
	ActionDeleteSlot         ActionKind = "delete_slot"
	ActionLaunch             ActionKind = "launch"
	ActionSetupAlias         ActionKind = "setup_alias"
	ActionSkipAliasSetup     ActionKind = "skip_alias_setup"
	ActionCopyAliasCommand   ActionKind = "copy_alias_command"
	ActionCloseAliasFallback ActionKind = "close_alias_fallback"
)

type Action struct {
	Kind         ActionKind
	AppID        string
	ProfileName  string
	ExistingName string
	Tag          string
	Provider     string
	APIKey       string
	BaseURL      string
	Model        string
	SmallModel   string
	Command      string
	Slot         int
}

type toolOption struct {
	ID          string
	Title       string
	Description string
	Enabled     bool
	Beta        bool
}

type model struct {
	state     State
	cfg       config.File
	store     *config.Store
	drivers   []driver.AppDriver
	driverMap map[string]driver.AppDriver

	mode   uiMode
	width  int
	height int

	tools     []toolOption
	toolIndex int

	slotCount      int
	slotSelection  int // 0..slotCount, slotCount = add-slot row
	profilesBySlot map[int]config.ProfileEntry

	addNameInput   textinput.Model
	addTagInput    textinput.Model
	addProvider    string
	addAPIKey      textinput.Model
	addBaseURL     textinput.Model
	addModel       textinput.Model
	addSmall       textinput.Model
	addEditing     bool
	addSourceName  string
	glmPresetIndex int
	addFocus       int
	addTarget      int

	deleteMode deleteTarget
	deleteSlot int

	action Action
}

func Run(state State, drivers []driver.AppDriver, store *config.Store) (Action, State, error) {
	cfg, err := store.Read()
	if err != nil {
		cfg = config.File{
			AutoSelectLastUsed: true,
			DefaultSlots:       config.DefaultSlotsCount,
		}
	}

	m := newModel(state, cfg, drivers, store)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	doneAny, err := prog.Run()
	if err != nil {
		return Action{}, state, err
	}
	done := doneAny.(*model)
	return done.action, done.exportState(), nil
}

func newModel(state State, cfg config.File, drivers []driver.AppDriver, store *config.Store) *model {
	m := &model{
		state:          state,
		cfg:            cfg,
		store:          store,
		drivers:        drivers,
		driverMap:      toDriverMap(drivers),
		tools:          buildToolOptions(drivers),
		toolIndex:      max(0, state.Selected),
		slotSelection:  max(0, state.Selected),
		profilesBySlot: map[int]config.ProfileEntry{},
	}
	if len(m.tools) == 0 {
		m.tools = []toolOption{{ID: "codex", Title: "Codex CLI", Description: "No drivers registered", Enabled: false}}
	}

	m.mode = modeFromScreen(resolveInitialScreen(cfg, state))
	m.state.ShowAliasPrompt = false
	m.state.AliasFallbackCommand = ""

	if m.mode == modeSlots || m.mode == modeAdd || m.mode == modeDeleteConfirm || m.mode == modeHelp {
		if state.CurrentAppID == "" || m.driverMap[state.CurrentAppID] == nil {
			state.CurrentAppID = m.defaultToolID()
		}
		m.state.CurrentAppID = state.CurrentAppID
		m.refreshSlots(true)
	}

	m.initAddInputs()
	m.expireStatusIfNeeded(time.Now().UTC())
	return m
}

func (m *model) Init() tea.Cmd { return statusTickCmd() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case statusTickMsg:
		m.expireStatusIfNeeded(time.Now().UTC())
		return m, statusTickCmd()
	case tea.KeyMsg:
		prevMsg := m.state.StatusMessage
		prevTS := m.state.StatusSetAtUnix
		switch m.mode {
		case modeWelcome:
			model, cmd := m.updateWelcome(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeTools:
			model, cmd := m.updateTools(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeSlots:
			model, cmd := m.updateSlots(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeAdd:
			model, cmd := m.updateAdd(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeDeleteConfirm:
			model, cmd := m.updateDeleteConfirm(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeHelp:
			model, cmd := m.updateHelp(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeAliasPrompt:
			model, cmd := m.updateAliasPrompt(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		case modeAliasFallback:
			model, cmd := m.updateAliasFallback(msg)
			m.updateStatusTimestamp(prevMsg, prevTS, time.Now().UTC())
			return model, cmd
		}
	}
	return m, nil
}

func (m *model) View() string {
	switch m.mode {
	case modeWelcome:
		return m.renderWelcome()
	case modeTools:
		return m.renderTools()
	case modeSlots:
		return m.renderSlots()
	case modeAdd:
		return m.renderAddForm()
	case modeDeleteConfirm:
		return m.renderDeleteConfirm()
	case modeHelp:
		return m.renderHelp()
	case modeAliasPrompt:
		return m.renderAliasPrompt()
	case modeAliasFallback:
		return m.renderAliasFallback()
	default:
		return m.renderTools()
	}
}

func (m *model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "ctrl+m":
		if err := m.store.SetOnboardingAccepted(true); err != nil {
			m.state.StatusMessage = fmt.Sprintf("Cannot save onboarding: %v", err)
		} else {
			m.cfg.OnboardingAccepted = true
			m.state.StatusMessage = "Welcome complete"
		}
		m.mode = modeTools
		m.state.Screen = ScreenToolPicker
		m.state.ShowAliasPrompt = false
		m.state.AliasFallbackCommand = ""
		if m.toolIndex < 0 || m.toolIndex >= len(m.tools) {
			m.toolIndex = 0
		}
		return m, nil
	case "q", "esc", "ctrl+c":
		return m.finish(Action{Kind: ActionQuit})
	}
	return m, nil
}

func (m *model) updateTools(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.toolIndex = clamp(m.toolIndex-1, 0, len(m.tools)-1)
	case "down", "j":
		m.toolIndex = clamp(m.toolIndex+1, 0, len(m.tools)-1)
	case "enter":
		tool := m.tools[m.toolIndex]
		if !tool.Enabled {
			m.state.StatusMessage = fmt.Sprintf("%s is not available in PATH", tool.Title)
			return m, nil
		}
		m.state.CurrentAppID = tool.ID
		m.mode = modeSlots
		m.refreshSlots(true)
	case "q", "esc", "ctrl+c":
		return m.finish(Action{Kind: ActionQuit})
	}
	return m, nil
}

func (m *model) updateSlots(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxSelection := max(0, m.slotCount) // add-slot is at index slotCount
	switch msg.String() {
	case "up", "k":
		m.slotSelection = clamp(m.slotSelection-1, 0, maxSelection)
	case "down", "j":
		m.slotSelection = clamp(m.slotSelection+1, 0, maxSelection)
	case "enter":
		if m.isAddSlotSelected() {
			return m.finish(Action{
				Kind:  ActionAddSlot,
				AppID: m.state.CurrentAppID,
			})
		}
		slot := m.selectedSlot()
		if p, ok := m.profilesBySlot[slot]; ok {
			return m.finish(Action{
				Kind:        ActionLaunch,
				AppID:       m.state.CurrentAppID,
				ProfileName: p.Name,
				Slot:        slot,
			})
		}
		m.mode = modeAdd
		m.addTarget = slot
		m.initAddInputs()
	case "a":
		if m.isAddSlotSelected() {
			return m.finish(Action{
				Kind:  ActionAddSlot,
				AppID: m.state.CurrentAppID,
			})
		}
		m.mode = modeAdd
		m.addTarget = m.selectedSlot()
		m.initAddInputs()
	case "e":
		if m.isAddSlotSelected() {
			m.state.StatusMessage = "Select a slot with account first"
			return m, nil
		}
		slot := m.selectedSlot()
		p, ok := m.profilesBySlot[slot]
		if !ok {
			m.state.StatusMessage = "Slot is empty"
			return m, nil
		}
		m.mode = modeAdd
		m.addTarget = slot
		m.initAddInputs()
		m.startEditForm(slot, p)
	case "d":
		if m.isAddSlotSelected() {
			return m, nil
		}
		slot := m.selectedSlot()
		if p, ok := m.profilesBySlot[slot]; ok {
			m.deleteMode = deleteProfile
			m.deleteSlot = slot
			m.addNameInput.SetValue(p.Name)
			m.mode = modeDeleteConfirm
			return m, nil
		}
		if m.slotCount <= config.DefaultSlotsCount {
			m.state.StatusMessage = fmt.Sprintf("Cannot delete below %d slots", config.DefaultSlotsCount)
			return m, nil
		}
		m.deleteMode = deleteSlot
		m.deleteSlot = slot
		m.mode = modeDeleteConfirm
	case "?", "h":
		m.mode = modeHelp
	case "q", "esc":
		m.mode = modeTools
		m.state.CurrentAppID = ""
		return m, nil
	}
	return m, nil
}

func (m *model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeSlots
		return m, nil
	case "tab", "shift+tab", "up", "down":
		maxFocus := m.addMaxFocus()
		if maxFocus <= 0 {
			return m, nil
		}
		if msg.String() == "shift+tab" || msg.String() == "up" {
			m.addFocus--
			if m.addFocus < 0 {
				m.addFocus = maxFocus
			}
		} else {
			m.addFocus++
			if m.addFocus > maxFocus {
				m.addFocus = 0
			}
		}
		m.updateAddFocus()
		return m, nil
	case "m", "left", "right":
		if m.isClaudeApp() {
			if m.addProvider == claudedrv.ProviderZAI {
				m.addProvider = claudedrv.ProviderAccount
			} else {
				m.addProvider = claudedrv.ProviderZAI
			}
			if m.addFocus > m.addMaxFocus() {
				m.addFocus = 0
			}
			if m.addProvider == claudedrv.ProviderZAI {
				m.ensureGLMDefaults()
				m.glmPresetIndex = detectGLMPreset(m.addModel.Value(), m.addSmall.Value())
			}
			m.updateAddFocus()
		}
		return m, nil
	case "g":
		if m.isClaudeApp() && m.addProvider == claudedrv.ProviderZAI {
			m.applyNextGLMPreset()
		}
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.addNameInput.Value())
		if m.addEditing {
			if name == "" {
				name = strings.TrimSpace(m.addSourceName)
			}
		} else if name == "" {
			auto, err := m.store.NextProfileName(m.state.CurrentAppID)
			if err != nil {
				m.state.StatusMessage = fmt.Sprintf("Cannot generate profile name: %v", err)
				m.mode = modeSlots
				return m, nil
			}
			name = auto
		}
		tag := strings.TrimSpace(m.addTagInput.Value())

		provider := ""
		apiKey := ""
		baseURL := ""
		model := ""
		smallModel := ""
		if m.isClaudeApp() {
			provider = m.addProvider
			if provider == claudedrv.ProviderZAI {
				apiKey = strings.TrimSpace(m.addAPIKey.Value())
				baseURL = strings.TrimSpace(m.addBaseURL.Value())
				model = strings.TrimSpace(m.addModel.Value())
				smallModel = strings.TrimSpace(m.addSmall.Value())
				if apiKey == "" {
					m.state.StatusMessage = "API key is required for Z.AI mode"
					return m, nil
				}
			}
		}

		kind := ActionAdd
		if m.addEditing {
			kind = ActionEdit
		}

		return m.finish(Action{
			Kind:         kind,
			AppID:        m.state.CurrentAppID,
			ProfileName:  name,
			ExistingName: m.addSourceName,
			Tag:          tag,
			Provider:     provider,
			APIKey:       apiKey,
			BaseURL:      baseURL,
			Model:        model,
			SmallModel:   smallModel,
			Slot:         m.addTarget,
		})
	}

	var cmd tea.Cmd
	switch m.addFocus {
	case 0:
		m.addNameInput, cmd = m.addNameInput.Update(msg)
	case 1:
		m.addTagInput, cmd = m.addTagInput.Update(msg)
	case 2:
		m.addAPIKey, cmd = m.addAPIKey.Update(msg)
	case 3:
		m.addBaseURL, cmd = m.addBaseURL.Update(msg)
	case 4:
		m.addModel, cmd = m.addModel.Update(msg)
	case 5:
		m.addSmall, cmd = m.addSmall.Update(msg)
	}
	if m.isClaudeApp() && m.addProvider == claudedrv.ProviderZAI {
		m.glmPresetIndex = detectGLMPreset(m.addModel.Value(), m.addSmall.Value())
	}
	return m, cmd
}

func (m *model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter", "d":
		if m.deleteMode == deleteProfile {
			if p, ok := m.profilesBySlot[m.deleteSlot]; ok {
				return m.finish(Action{
					Kind:        ActionDelete,
					AppID:       m.state.CurrentAppID,
					ProfileName: p.Name,
					Slot:        m.deleteSlot,
				})
			}
			m.mode = modeSlots
			return m, nil
		}
		if m.deleteMode == deleteSlot {
			return m.finish(Action{
				Kind:  ActionDeleteSlot,
				AppID: m.state.CurrentAppID,
				Slot:  m.deleteSlot,
			})
		}
	case "n", "q", "esc":
		m.mode = modeSlots
	}
	return m, nil
}

func (m *model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q", "esc", "ctrl+c":
		m.mode = modeSlots
	}
	return m, nil
}

func (m *model) updateAliasPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		return m.finish(Action{Kind: ActionSetupAlias})
	case "n", "s", "q", "esc", "ctrl+c":
		return m.finish(Action{Kind: ActionSkipAliasSetup})
	}
	return m, nil
}

func (m *model) updateAliasFallback(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		return m.finish(Action{
			Kind:    ActionCopyAliasCommand,
			Command: m.state.AliasFallbackCommand,
		})
	case "enter", "q", "esc", "ctrl+c":
		return m.finish(Action{Kind: ActionCloseAliasFallback})
	}
	return m, nil
}

func (m *model) finish(action Action) (tea.Model, tea.Cmd) {
	m.state.Selected = m.selectionForCurrentMode()
	m.action = action
	return m, tea.Quit
}

func (m *model) exportState() State {
	out := m.state
	switch m.mode {
	case modeWelcome:
		out.Screen = ScreenWelcome
		out.CurrentAppID = ""
		out.Selected = 0
	case modeTools:
		out.Screen = ScreenToolPicker
		out.CurrentAppID = ""
		out.Selected = m.toolIndex
	default:
		out.Screen = ScreenAccountSlots
		out.Selected = m.slotSelection
	}
	return out
}

func statusTickCmd() tea.Cmd {
	return tea.Tick(statusTickInterval, func(_ time.Time) tea.Msg {
		return statusTickMsg{}
	})
}

func (m *model) updateStatusTimestamp(prevMsg string, prevTS int64, now time.Time) {
	curMsg := strings.TrimSpace(m.state.StatusMessage)
	if curMsg == "" {
		m.state.StatusSetAtUnix = 0
		return
	}
	if curMsg != strings.TrimSpace(prevMsg) || prevTS == 0 {
		m.state.StatusSetAtUnix = now.Unix()
	}
}

func (m *model) expireStatusIfNeeded(now time.Time) {
	msg := strings.TrimSpace(m.state.StatusMessage)
	if msg == "" {
		m.state.StatusSetAtUnix = 0
		return
	}
	if m.state.StatusSetAtUnix == 0 {
		m.state.StatusSetAtUnix = now.Unix()
		return
	}
	if now.Unix()-m.state.StatusSetAtUnix >= int64(statusTTL/time.Second) {
		m.state.StatusMessage = ""
		m.state.StatusSetAtUnix = 0
	}
}

func (m *model) selectionForCurrentMode() int {
	if m.mode == modeTools {
		return m.toolIndex
	}
	return m.slotSelection
}

func (m *model) refreshSlots(preferLastUsed bool) {
	slotCount, err := m.store.SlotCount(m.state.CurrentAppID)
	if err != nil {
		slotCount = config.DefaultSlotsCount
		m.state.StatusMessage = fmt.Sprintf("Read slots failed: %v", err)
	}
	m.slotCount = max(config.DefaultSlotsCount, slotCount)

	profiles, err := m.store.ListProfiles(m.state.CurrentAppID)
	if err != nil {
		m.state.StatusMessage = fmt.Sprintf("Read profiles failed: %v", err)
		profiles = nil
	}

	m.profilesBySlot = make(map[int]config.ProfileEntry, len(profiles))
	for _, p := range profiles {
		if p.Slot > 0 {
			m.profilesBySlot[p.Slot] = p
		}
	}

	if preferLastUsed && m.cfg.AutoSelectLastUsed {
		if lastName, ok, err := m.store.LastUsedProfileName(m.state.CurrentAppID); err == nil && ok {
			for slot, p := range m.profilesBySlot {
				if p.Name == lastName {
					m.slotSelection = slot - 1
					return
				}
			}
		}
	}
	m.slotSelection = clamp(m.slotSelection, 0, m.slotCount)
}

func (m *model) initAddInputs() {
	m.addNameInput = textinput.New()
	m.addNameInput.Placeholder = "auto if empty"
	m.addNameInput.CharLimit = 60
	m.addNameInput.Width = 24

	m.addTagInput = textinput.New()
	m.addTagInput.Placeholder = "work / personal / team"
	m.addTagInput.CharLimit = 24
	m.addTagInput.Width = 24

	m.addProvider = claudedrv.ProviderAccount
	m.addEditing = false
	m.addSourceName = ""

	m.addAPIKey = textinput.New()
	m.addAPIKey.Placeholder = "required for Z.AI mode"
	m.addAPIKey.CharLimit = 180
	m.addAPIKey.Width = 42
	m.addAPIKey.EchoMode = textinput.EchoPassword
	m.addAPIKey.EchoCharacter = '•'

	m.addBaseURL = textinput.New()
	m.addBaseURL.Placeholder = claudedrv.DefaultZAIBaseURL
	m.addBaseURL.SetValue(claudedrv.DefaultZAIBaseURL)
	m.addBaseURL.CharLimit = 180
	m.addBaseURL.Width = 42

	m.addModel = textinput.New()
	m.addModel.Placeholder = claudedrv.DefaultZAIModel
	m.addModel.SetValue(claudedrv.DefaultZAIModel)
	m.addModel.CharLimit = 80
	m.addModel.Width = 42

	m.addSmall = textinput.New()
	m.addSmall.Placeholder = claudedrv.DefaultZAISmallFast
	m.addSmall.SetValue(claudedrv.DefaultZAISmallFast)
	m.addSmall.CharLimit = 80
	m.addSmall.Width = 42

	m.glmPresetIndex = detectGLMPreset(m.addModel.Value(), m.addSmall.Value())
	m.addFocus = 0
	m.updateAddFocus()
}

func (m *model) updateAddFocus() {
	if m.addFocus > m.addMaxFocus() {
		m.addFocus = 0
	}
	m.addNameInput.Blur()
	m.addTagInput.Blur()
	m.addAPIKey.Blur()
	m.addBaseURL.Blur()
	m.addModel.Blur()
	m.addSmall.Blur()

	switch m.addFocus {
	case 0:
		m.addNameInput.Focus()
	case 1:
		m.addTagInput.Focus()
	case 2:
		m.addAPIKey.Focus()
	case 3:
		m.addBaseURL.Focus()
	case 4:
		m.addModel.Focus()
	case 5:
		m.addSmall.Focus()
	}
}

func (m *model) addMaxFocus() int {
	if !m.isClaudeApp() {
		return 1
	}
	if m.addProvider == claudedrv.ProviderZAI {
		return 5
	}
	return 1
}

func (m *model) isClaudeApp() bool {
	return m.state.CurrentAppID == "claude"
}

func (m *model) startEditForm(slot int, p config.ProfileEntry) {
	m.addEditing = true
	m.addSourceName = p.Name
	m.addTarget = slot

	m.addNameInput.SetValue(p.Name)
	m.addTagInput.SetValue(p.Tag)

	if !m.isClaudeApp() {
		m.updateAddFocus()
		return
	}

	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider != claudedrv.ProviderZAI {
		provider = claudedrv.ProviderAccount
	}
	m.addProvider = provider

	if strings.TrimSpace(p.BaseURL) != "" {
		m.addBaseURL.SetValue(strings.TrimSpace(p.BaseURL))
	}
	if strings.TrimSpace(p.Model) != "" {
		m.addModel.SetValue(strings.TrimSpace(p.Model))
	}

	profileDir, err := m.store.ProfileDir(m.state.CurrentAppID, p.Name)
	if err == nil {
		if settings, err := claudedrv.LoadProfileSettings(profileDir); err == nil {
			m.addProvider = settings.Provider
			if strings.TrimSpace(settings.APIKey) != "" {
				m.addAPIKey.SetValue(settings.APIKey)
			}
			if strings.TrimSpace(settings.BaseURL) != "" {
				m.addBaseURL.SetValue(settings.BaseURL)
			}
			if strings.TrimSpace(settings.Model) != "" {
				m.addModel.SetValue(settings.Model)
			}
			if strings.TrimSpace(settings.SmallModel) != "" {
				m.addSmall.SetValue(settings.SmallModel)
			}
		}
	}

	m.ensureGLMDefaults()
	m.glmPresetIndex = detectGLMPreset(m.addModel.Value(), m.addSmall.Value())
	m.updateAddFocus()
}

func (m *model) ensureGLMDefaults() {
	if strings.TrimSpace(m.addBaseURL.Value()) == "" {
		m.addBaseURL.SetValue(claudedrv.DefaultZAIBaseURL)
	}
	if strings.TrimSpace(m.addModel.Value()) == "" {
		m.addModel.SetValue(claudedrv.DefaultZAIModel)
	}
	if strings.TrimSpace(m.addSmall.Value()) == "" {
		m.addSmall.SetValue(claudedrv.DefaultZAISmallFast)
	}
}

func (m *model) applyNextGLMPreset() {
	presets := glmPresets()
	if len(presets) == 0 {
		return
	}
	if m.glmPresetIndex < 0 || m.glmPresetIndex >= len(presets) {
		m.glmPresetIndex = 0
	} else {
		m.glmPresetIndex = (m.glmPresetIndex + 1) % len(presets)
	}
	p := presets[m.glmPresetIndex]
	m.addModel.SetValue(p.Model)
	m.addSmall.SetValue(p.SmallModel)
}

func (m *model) currentGLMPresetLabel() string {
	presets := glmPresets()
	if m.glmPresetIndex >= 0 && m.glmPresetIndex < len(presets) {
		return presets[m.glmPresetIndex].Label
	}
	return "Custom"
}

type glmPreset struct {
	Label      string
	Model      string
	SmallModel string
}

func glmPresets() []glmPreset {
	return []glmPreset{
		{Label: "Balanced", Model: claudedrv.DefaultZAIModel, SmallModel: claudedrv.DefaultZAISmallFast},
		{Label: "Coding", Model: "glm-4.7", SmallModel: claudedrv.DefaultZAISmallFast},
		{Label: "Max", Model: "glm-5", SmallModel: claudedrv.DefaultZAISmallFast},
	}
}

func detectGLMPreset(model, smallModel string) int {
	model = strings.ToLower(strings.TrimSpace(model))
	smallModel = strings.ToLower(strings.TrimSpace(smallModel))
	for i, p := range glmPresets() {
		if model == strings.ToLower(strings.TrimSpace(p.Model)) &&
			smallModel == strings.ToLower(strings.TrimSpace(p.SmallModel)) {
			return i
		}
	}
	return -1
}

func (m *model) selectedSlot() int {
	return m.slotSelection + 1
}

func (m *model) isAddSlotSelected() bool {
	return m.slotSelection == m.slotCount
}

func (m *model) renderWelcome() string {
	content := panelStyle().Width(viewWidth(m.width, 70)).Render(
		logoStyle().Render(swittcherLogo()) + "\n\n" +
			bodyStyle().Render("Official login flows. Isolated profiles. No token sharing.\nProvider limits/policies are controlled by providers.") + "\n\n" +
			hintStyle().Render("[enter] Continue   [q] Exit"),
	)
	return layoutCenter(content, m.width, m.height)
}

func (m *model) renderTools() string {
	lines := make([]string, 0, len(m.tools))
	for i, t := range m.tools {
		lines = append(lines, m.renderToolRow(i, t))
	}

	body := panelStyle().Width(viewWidth(m.width, 70)).Render(
		logoStyle().Render(swittcherLogo()) + "\n\n" +
			strings.Join(lines, "\n") + "\n\n" +
			renderStatusLine(m.state.StatusMessage, "[enter] Select   [j/k] Move   [q] Quit"),
	)
	return layoutCenter(body, m.width, m.height)
}

func (m *model) renderSlots() string {
	slotsHeader := titleStyle().Render(fmt.Sprintf("%s Slots", m.currentToolTitle()))
	if isBetaTool(m.state.CurrentAppID) {
		slotsHeader = lipgloss.JoinHorizontal(lipgloss.Left, slotsHeader, " ", betaBadgeStyle().Render("BETA"))
	}
	lines := []string{slotsHeader}

	for slot := 1; slot <= m.slotCount; slot++ {
		selected := m.slotSelection == slot-1
		prefix := "  "
		if selected {
			prefix = "> "
		}
		p, ok := m.profilesBySlot[slot]
		badge := "[EMPTY]"
		row := fmt.Sprintf("Slot %d %s", slot, badge)
		if ok {
			badge = "[SET]"
			if strings.TrimSpace(p.Tag) != "" {
				badge = "[" + strings.ToUpper(p.Tag) + "]"
			}
			row = fmt.Sprintf("Slot %d %s %s", slot, badge, p.Name)
		}
		lines = append(lines, sidebarItemStyle(selected).Render(prefix+row))
	}

	addPrefix := "  "
	if m.isAddSlotSelected() {
		addPrefix = "> "
	}
	lines = append(lines, sidebarItemStyle(m.isAddSlotSelected()).Render(addPrefix+"Add slot"))

	selectedTitle, selectedMeta := m.selectedSlotSummary()
	lines = append(
		lines,
		"",
		bodyStyle().Render(selectedTitle),
		bodyStyle().Render(strings.Join(selectedMeta, "\n")),
		"",
		renderStatusLine(m.state.StatusMessage, "[enter] launch/add   [a] add   [e] edit   [d] delete   [?] help   [q] back"),
	)

	content := panelStyle().
		Width(viewWidth(m.width, 90)).
		Render(strings.Join(lines, "\n"))
	return layoutCenter(content, m.width, m.height)
}

func (m *model) selectedSlotSummary() (string, []string) {
	if m.isAddSlotSelected() {
		return "Selected: Add slot", []string{
			"Create one more empty slot.",
			fmt.Sprintf("Current slots: %d", m.slotCount),
		}
	}

	slot := m.selectedSlot()
	if p, ok := m.profilesBySlot[slot]; ok {
		details := nonEmpty(
			"Ready to launch this account.",
			detailLine("Email", p.Email),
			detailLine("Plan", p.Plan),
			detailLine("Provider", p.Provider),
			detailLine("Base URL", p.BaseURL),
			detailLine("Model", p.Model),
			detailLine("Tag", p.Tag),
			detailLine("Account", p.AccountID),
		)
		if len(details) == 0 {
			details = []string{"No metadata yet"}
		}
		return fmt.Sprintf("Selected: Slot %d · %s", slot, p.Name), details
	}

	return fmt.Sprintf("Selected: Slot %d", slot), []string{
		"Empty slot. Press Enter to bind an account.",
	}
}

func (m *model) renderSidebar() string {
	lines := make([]string, 0, m.slotCount+3)
	sidebarHeader := sidebarSectionStyle().Render(m.currentToolTitle())
	if isBetaTool(m.state.CurrentAppID) {
		sidebarHeader = lipgloss.JoinHorizontal(lipgloss.Left, sidebarHeader, " ", betaBadgeStyle().Render("BETA"))
	}
	lines = append(lines, sidebarHeader)
	for slot := 1; slot <= m.slotCount; slot++ {
		selected := m.slotSelection == slot-1
		p, ok := m.profilesBySlot[slot]
		badge := "[EMPTY]"
		if ok {
			if strings.TrimSpace(p.Tag) != "" {
				badge = "[" + strings.ToUpper(p.Tag) + "]"
			} else {
				badge = "[SET]"
			}
		}
		line := fmt.Sprintf("- Slot %d %s", slot, badge)
		lines = append(lines, sidebarItemStyle(selected).Render(line))
	}
	lines = append(lines, sidebarItemStyle(m.isAddSlotSelected()).Render("- Add slot"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		logoStyle().Render(swittcherLogo()),
		"",
		strings.Join(lines, "\n"),
		"",
		hintMutedStyle().Render("[j/k] move  [enter] select"),
	)
	return panelStyle().
		Width(sidebarWidth(m.width)).
		Render(content)
}

func (m *model) renderSlotDetails() string {
	var title, desc, meta string
	titleWithBadge := ""
	selectedAdd := m.isAddSlotSelected()
	if selectedAdd {
		title = "Add slot"
		desc = "Create one more empty slot."
		meta = fmt.Sprintf("Current slots: %d", m.slotCount)
	} else {
		slot := m.selectedSlot()
		if p, ok := m.profilesBySlot[slot]; ok {
			title = fmt.Sprintf("Slot %d · %s", slot, p.Name)
			desc = "Account is bound to this slot."
			details := nonEmpty(
				detailLine("Email", p.Email),
				detailLine("Plan", p.Plan),
				detailLine("Provider", p.Provider),
				detailLine("Base URL", p.BaseURL),
				detailLine("Model", p.Model),
				detailLine("Tag", p.Tag),
				detailLine("Account", p.AccountID),
			)
			if len(details) == 0 {
				details = []string{"No metadata yet"}
			}
			meta = "✓ Ready\n" + strings.Join(details, "\n")
		} else {
			title = fmt.Sprintf("Slot %d", slot)
			desc = "Empty slot. Press Enter to bind an account."
			meta = "No account attached."
		}
	}
	titleWithBadge = titleStyle().Render(title)
	if !selectedAdd && isBetaTool(m.state.CurrentAppID) {
		titleWithBadge = lipgloss.JoinHorizontal(lipgloss.Left, titleWithBadge, " ", betaBadgeStyle().Render("BETA"))
	}

	help := "[enter] launch/add   [a] add   [d] delete   [?] help   [q] back"
	content := titleWithBadge + "\n" + bodyStyle().Render(desc) + "\n\n" + bodyStyle().Render(meta) + "\n\n" + renderStatusLine(m.state.StatusMessage, help)
	return panelStyle().
		Width(detailsWidth(m.width)).
		Render(content)
}

func (m *model) renderAddForm() string {
	slot := m.addTarget
	title := fmt.Sprintf("Bind Account to Slot %d", slot)
	nameLabel := "Profile Name (optional)"
	if m.addEditing {
		title = fmt.Sprintf("Edit Slot %d", slot)
		nameLabel = "Profile Name"
	}

	text := titleStyle().Render(title) + "\n\n" +
		fieldLabelStyle(m.addFocus == 0).Render(nameLabel) + "\n" +
		m.addNameInput.View() + "\n\n" +
		fieldLabelStyle(m.addFocus == 1).Render("Tag (optional)") + "\n" +
		m.addTagInput.View()

	if m.isClaudeApp() {
		modeLabel := "Anthropic account"
		if m.addProvider == claudedrv.ProviderZAI {
			modeLabel = "Z.AI API gateway"
		}
		text += "\n\n" +
			bodyStyle().Render("Auth mode: "+modeLabel) + "\n" +
			hintMutedStyle().Render("[m] toggle mode")
		if m.addProvider == claudedrv.ProviderZAI {
			text += "\n\n" +
				bodyStyle().Render("GLM preset: "+m.currentGLMPresetLabel()) + "\n" +
				hintMutedStyle().Render("[g] cycle preset") + "\n\n" +
				fieldLabelStyle(m.addFocus == 2).Render("Z.AI API key") + "\n" +
				m.addAPIKey.View() + "\n\n" +
				fieldLabelStyle(m.addFocus == 3).Render("Base URL") + "\n" +
				m.addBaseURL.View() + "\n\n" +
				fieldLabelStyle(m.addFocus == 4).Render("Model") + "\n" +
				m.addModel.View() + "\n\n" +
				fieldLabelStyle(m.addFocus == 5).Render("Small/Fast Model") + "\n" +
				m.addSmall.View()
		}
	}

	text += "\n\n" + hintStyle().Render("[tab] switch   [enter] save   [esc] cancel")
	targetWidth := 58
	if m.isClaudeApp() {
		targetWidth = 86
	}
	return layoutCenter(panelStyle().Width(viewWidth(m.width, targetWidth)).Render(text), m.width, m.height)
}

func (m *model) renderDeleteConfirm() string {
	title := "Confirm Delete"
	msg := ""
	switch m.deleteMode {
	case deleteProfile:
		p := m.profilesBySlot[m.deleteSlot]
		msg = fmt.Sprintf("Delete account %q from Slot %d?\n\nThis keeps Slot %d empty.\n\n[y] confirm   [n] cancel", p.Name, m.deleteSlot, m.deleteSlot)
	case deleteSlot:
		msg = fmt.Sprintf("Delete empty Slot %d?\n\n[y] confirm   [n] cancel", m.deleteSlot)
	default:
		msg = "Nothing selected."
	}
	return layoutCenter(panelStyle().Width(viewWidth(m.width, 56)).Render(titleStyle().Render(title)+"\n\n"+bodyStyle().Render(msg)), m.width, m.height)
}

func (m *model) renderHelp() string {
	msg := strings.Join([]string{
		"Slots screen:",
		"  j/k       Move",
		"  Enter     Launch/Add slot action",
		"  a         Add account in selected slot",
		"  e         Edit selected account",
		"  d         Delete account or empty slot",
		"  ?         Help",
		"  q / esc   Back",
		"",
		"Delete logic:",
		"  Account slot -> removes account, slot stays empty",
		"  Empty slot   -> removes slot (minimum 4 slots)",
	}, "\n")
	return layoutCenter(panelStyle().Width(viewWidth(m.width, 70)).Render(titleStyle().Render("Help")+"\n\n"+bodyStyle().Render(msg)), m.width, m.height)
}

func (m *model) renderAliasPrompt() string {
	aliasName, preview := aliasPreviewForApp(m.state.CurrentAppID)
	msg := strings.Join([]string{
		fmt.Sprintf("Create alias `%s` for quick start?", aliasName),
		"",
		preview,
		"",
		"[y/enter] setup   [n] skip",
	}, "\n")
	return layoutCenter(panelStyle().Width(viewWidth(m.width, 60)).Render(titleStyle().Render("Alias Setup")+"\n\n"+bodyStyle().Render(msg)), m.width, m.height)
}

func (m *model) renderAliasFallback() string {
	msg := "Auto setup failed.\n\nRun this command manually:\n\n" + m.state.AliasFallbackCommand + "\n\n[c] copy   [enter] close"
	return layoutCenter(panelStyle().Width(viewWidth(m.width, 92)).Render(titleStyle().Render("Alias Fallback")+"\n\n"+bodyStyle().Render(msg)), m.width, m.height)
}

func resolveInitialScreen(cfg config.File, state State) Screen {
	if !cfg.OnboardingAccepted {
		return ScreenWelcome
	}
	if state.Screen == ScreenAccountSlots && strings.TrimSpace(state.CurrentAppID) != "" {
		return ScreenAccountSlots
	}
	return ScreenToolPicker
}

func modeFromScreen(screen Screen) uiMode {
	switch screen {
	case ScreenWelcome:
		return modeWelcome
	case ScreenAccountSlots:
		return modeSlots
	default:
		return modeTools
	}
}

func toDriverMap(drivers []driver.AppDriver) map[string]driver.AppDriver {
	out := make(map[string]driver.AppDriver, len(drivers))
	for _, d := range drivers {
		out[d.ID()] = d
	}
	return out
}

func buildToolOptions(drivers []driver.AppDriver) []toolOption {
	out := make([]toolOption, 0, len(drivers))
	for _, d := range drivers {
		enabled := d.IsAvailable()
		desc := ""
		if !enabled {
			desc = "CLI not found in PATH"
		}
		out = append(out, toolOption{
			ID:          d.ID(),
			Title:       d.DisplayName(),
			Description: desc,
			Enabled:     enabled,
			Beta:        isBetaTool(d.ID()),
		})
	}
	return out
}

func (m *model) renderToolRow(i int, t toolOption) string {
	prefix := "  "
	if i == m.toolIndex {
		prefix = "> "
	}

	title := titleStyle().Render(t.Title)
	if t.Beta {
		title = lipgloss.JoinHorizontal(lipgloss.Left, title, " ", betaBadgeStyle().Render("BETA"))
	}

	parts := []string{prefix, title}
	if strings.TrimSpace(t.Description) != "" {
		status := toolStatusBadgeStyle(t.Enabled).Render(t.Description)
		parts = append(parts, "  ", status)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Left, parts...)
	if i == m.toolIndex {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#273447")).
			Foreground(lipgloss.Color("#E8EDF5")).
			Render(row)
	}
	return row
}

func (m *model) defaultToolID() string {
	if _, ok := m.driverMap["codex"]; ok {
		return "codex"
	}
	if len(m.tools) > 0 {
		return m.tools[0].ID
	}
	return ""
}

func (m *model) currentToolTitle() string {
	if d, ok := m.driverMap[m.state.CurrentAppID]; ok {
		return d.DisplayName()
	}
	return "Tool"
}

func aliasPreviewForApp(appID string) (aliasName, preview string) {
	switch strings.ToLower(strings.TrimSpace(appID)) {
	case "claude":
		return "cc", "cc -> swittcher --claude"
	default:
		return "cx", "cx -> swittcher --codex"
	}
}

func isBetaTool(appID string) bool {
	switch strings.ToLower(strings.TrimSpace(appID)) {
	case "claude", "gemini":
		return true
	default:
		return false
	}
}

func shouldPromptAliasFromConfig(cfg config.File, appID string) bool {
	// Temporary kill switch: disable auto alias setup/prompt flow.
	return false
}

func detailLine(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func sidebarWidth(total int) int {
	if total <= 0 {
		return 32
	}
	w := total / 3
	return clamp(w, 28, 38)
}

func detailsWidth(total int) int {
	if total <= 0 {
		return 64
	}
	left := sidebarWidth(total)
	return max(44, total-left-2)
}

func panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#5A6474")).
		Padding(0, 1)
}

func titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EDF5"))
}

func logoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#A5B9D6"))
}

func bodyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
}

func hintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8FB6D8"))
}

func hintMutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#7B8797"))
}

func betaBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#2D1B00")).
		Background(lipgloss.Color("#F4C36A")).
		Padding(0, 1)
}

func toolStatusBadgeStyle(enabled bool) lipgloss.Style {
	if enabled {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0A2A16")).
			Background(lipgloss.Color("#89D6A7")).
			Padding(0, 1)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D2D8E0")).
		Background(lipgloss.Color("#435062")).
		Padding(0, 1)
}

func sidebarSectionStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E8EDF5"))
}

func sidebarItemStyle(selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#CAD2DD"))
	if selected {
		style = style.Foreground(lipgloss.Color("#111418")).Background(lipgloss.Color("#9BB3CF")).Bold(true)
	}
	return style
}

func fieldLabelStyle(focused bool) lipgloss.Style {
	if focused {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#A7CFA0"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8FA0B2"))
}

func renderStatusLine(status, fallback string) string {
	msg := strings.TrimSpace(status)
	if msg == "" {
		msg = fallback
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#8BA3BC")).Render(msg)
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "fail"), strings.Contains(lower, "error"), strings.Contains(lower, "cannot"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E57373")).Render("✗ " + msg)
	case strings.Contains(lower, "added"), strings.Contains(lower, "deleted"), strings.Contains(lower, "configured"), strings.Contains(lower, "copied"), strings.Contains(lower, "complete"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#81C784")).Render("✓ " + msg)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#8BA3BC")).Render("• " + msg)
	}
}

func layoutCenter(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return content
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func viewWidth(width, target int) int {
	if width <= 0 {
		return target
	}
	return clamp(width-4, 30, target)
}

func swittcherLogo() string {
	return strings.Trim(`
 _____       _ _   _      _
/  ___|     (_) | | |    | |
\ `+"`"+`--. _ __  _| |_| |_ __| |__   ___ _ __
 `+"`"+`--. \ '_ \| | __| __/ _' '_ \ / _ \ '__|
/\__/ / | | | | |_| || (_| | | |  __/ |
\____/|_| |_|_|\__|\__\__,_| |_|\___|_|
`, "\n")
}

func clamp(v, minV, maxV int) int {
	if maxV < minV {
		return minV
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
