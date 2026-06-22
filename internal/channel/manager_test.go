package channel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"SuperBotGo/internal/errs"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockUserService struct {
	FindOrCreateUserFn func(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, username ...string) (*model.GlobalUser, error)
	GetUserFn          func(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error)
}

func (m *mockUserService) FindOrCreateUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, username ...string) (*model.GlobalUser, error) {
	if m.FindOrCreateUserFn != nil {
		return m.FindOrCreateUserFn(ctx, channelType, platformUserID, username...)
	}
	return &model.GlobalUser{ID: 1, Locale: "en"}, nil
}

func (m *mockUserService) GetUser(ctx context.Context, id model.GlobalUserID) (*model.GlobalUser, error) {
	if m.GetUserFn != nil {
		return m.GetUserFn(ctx, id)
	}
	return &model.GlobalUser{ID: id, Locale: "en"}, nil
}

type mockStateManager struct {
	RegisterFn              func(pluginID string, def *state.CommandDefinition)
	StartCommandFn          func(ctx context.Context, userID model.GlobalUserID, chatID string, pluginID string, commandName string, locale string) (*StateResult, error)
	ProcessInputFn          func(ctx context.Context, userID model.GlobalUserID, chatID string, input model.UserInput, locale string) (*StateResult, error)
	CancelCommandFn         func(ctx context.Context, userID model.GlobalUserID) error
	IsPreservesDialogFn     func(pluginID, commandName string) bool
	GetCurrentStepMessageFn func(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error)
}

func (m *mockStateManager) Register(pluginID string, def *state.CommandDefinition) {
	if m.RegisterFn != nil {
		m.RegisterFn(pluginID, def)
	}
}

func (m *mockStateManager) StartCommand(ctx context.Context, userID model.GlobalUserID, chatID string, pluginID string, commandName string, locale string) (*StateResult, error) {
	if m.StartCommandFn != nil {
		return m.StartCommandFn(ctx, userID, chatID, pluginID, commandName, locale)
	}
	return &StateResult{}, nil
}

func (m *mockStateManager) ProcessInput(ctx context.Context, userID model.GlobalUserID, chatID string, input model.UserInput, locale string) (*StateResult, error) {
	if m.ProcessInputFn != nil {
		return m.ProcessInputFn(ctx, userID, chatID, input, locale)
	}
	return &StateResult{}, nil
}

func (m *mockStateManager) CancelCommand(ctx context.Context, userID model.GlobalUserID) error {
	if m.CancelCommandFn != nil {
		return m.CancelCommandFn(ctx, userID)
	}
	return nil
}

func (m *mockStateManager) IsPreservesDialog(pluginID, commandName string) bool {
	if m.IsPreservesDialogFn != nil {
		return m.IsPreservesDialogFn(pluginID, commandName)
	}
	return false
}

func (m *mockStateManager) GetCurrentStepMessage(ctx context.Context, userID model.GlobalUserID, locale string) (*model.Message, string, error) {
	if m.GetCurrentStepMessageFn != nil {
		return m.GetCurrentStepMessageFn(ctx, userID, locale)
	}
	return nil, "", nil
}

type mockPluginRegistry struct {
	GetCommandDefinitionFn func(commandName string) *state.CommandDefinition
	GetPluginIDByCommandFn func(commandName string) string
	ResolveCommandFn       func(input string) (pluginID string, def *state.CommandDefinition, candidates []model.CommandCandidate)
}

func (m *mockPluginRegistry) GetCommandDefinition(commandName string) *state.CommandDefinition {
	if m.GetCommandDefinitionFn != nil {
		return m.GetCommandDefinitionFn(commandName)
	}
	return nil
}

func (m *mockPluginRegistry) GetPluginIDByCommand(commandName string) string {
	if m.GetPluginIDByCommandFn != nil {
		return m.GetPluginIDByCommandFn(commandName)
	}
	return ""
}

func (m *mockPluginRegistry) ResolveCommand(input string) (pluginID string, def *state.CommandDefinition, candidates []model.CommandCandidate) {
	if m.ResolveCommandFn != nil {
		return m.ResolveCommandFn(input)
	}
	return "", nil, nil
}

type mockEventRouter struct {
	RouteEventFn func(ctx context.Context, event contract.Event) (*contract.EventResponse, error)
}

func (m *mockEventRouter) RouteEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error) {
	if m.RouteEventFn != nil {
		return m.RouteEventFn(ctx, event)
	}
	return &contract.EventResponse{}, nil
}

type mockAuthorizer struct {
	CheckCommandFn func(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error)
}

func (m *mockAuthorizer) CheckCommand(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error) {
	if m.CheckCommandFn != nil {
		return m.CheckCommandFn(ctx, userID, pluginID, commandName, requirements)
	}
	return true, nil
}

type mockFocusTracker struct {
	mu         sync.Mutex
	recorded   []string
	lastPlugin string
}

func (m *mockFocusTracker) Record(userID model.GlobalUserID, pluginID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recorded = append(m.recorded, pluginID)
	m.lastPlugin = pluginID
}

func (m *mockFocusTracker) LastPlugin(userID model.GlobalUserID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastPlugin
}

// mockChannelAdapter records messages sent via SendToChat / SendToUser.
type mockChannelAdapter struct {
	mu       sync.Mutex
	chatMsgs []sentMessage
	userMsgs []sentMessage
}

type sentMessage struct {
	target string // chatID or platformUserID
	msg    model.Message
}

func (a *mockChannelAdapter) Type() model.ChannelType { return model.ChannelTelegram }

func (a *mockChannelAdapter) SendToChat(_ context.Context, chatID string, msg model.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.chatMsgs = append(a.chatMsgs, sentMessage{target: chatID, msg: msg})
	return nil
}

func (a *mockChannelAdapter) SendToUser(_ context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.userMsgs = append(a.userMsgs, sentMessage{target: string(platformUserID), msg: msg})
	return nil
}

func (a *mockChannelAdapter) chatMessages() []sentMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := make([]sentMessage, len(a.chatMsgs))
	copy(cp, a.chatMsgs)
	return cp
}

// ---------------------------------------------------------------------------
// Helper to build a ready-to-use ChannelManager with defaults.
// ---------------------------------------------------------------------------

type testDeps struct {
	userService *mockUserService
	state       *mockStateManager
	plugins     *mockPluginRegistry
	router      *mockEventRouter
	authorizer  *mockAuthorizer
	adapter     *mockChannelAdapter
	focus       *mockFocusTracker
}

func newTestManager() (*ChannelManager, *testDeps) {
	deps := &testDeps{
		userService: &mockUserService{},
		state:       &mockStateManager{},
		plugins:     &mockPluginRegistry{},
		router:      &mockEventRouter{},
		authorizer:  &mockAuthorizer{},
		adapter:     &mockChannelAdapter{},
		focus:       &mockFocusTracker{},
	}

	reg := NewAdapterRegistry()
	reg.Register(deps.adapter)

	mgr := NewChannelManager(
		deps.userService,
		deps.router,
		deps.state,
		deps.plugins,
		deps.authorizer,
		reg,
		deps.focus,
		nil, // logger — uses slog.Default()
	)
	return mgr, deps
}

func makeUpdate(text string) Update {
	return Update{
		ChannelType:    model.ChannelTelegram,
		PlatformUserID: "user123",
		Input:          model.TextInput{Text: text},
		ChatID:         "chat42",
		Username:       "testuser",
	}
}

// firstTextBlock extracts the text of the first TextBlock in a message, or "".
func firstTextBlock(msg model.Message) string {
	for _, b := range msg.Blocks {
		if tb, ok := b.(model.TextBlock); ok {
			return tb.Text
		}
	}
	return ""
}

func firstOptionsBlock(msg model.Message) *model.OptionsBlock {
	for _, b := range msg.Blocks {
		if ob, ok := b.(model.OptionsBlock); ok {
			return &ob
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests for OnUpdate — text command
// ---------------------------------------------------------------------------

func TestOnUpdate_TextCommand_ResolvesAndRoutes(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "ping"}
	deps.plugins.ResolveCommandFn = func(input string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		if input == "ping" {
			return "pluginA", def, nil
		}
		return "", nil, nil
	}

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, event)
		if len(routed) == 1 && event.PluginID != "pluginA" {
			t.Errorf("expected first pluginID %q, got %q", "pluginA", event.PluginID)
		}
		return &contract.EventResponse{}, nil
	}

	// StartCommand returns immediately complete (no steps).
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "ping",
			IsComplete:  true,
			Params:      model.OptionMap{"key": "val"},
		}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/ping"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routed) == 0 {
		t.Error("expected event to be routed")
	}
	if len(routed) != 1 {
		t.Fatalf("expected only command route; plugin menu is sent through state, got %d event(s)", len(routed))
	}
}

func TestOnUpdate_TextInput_ProcessesViaStateManager(t *testing.T) {
	mgr, deps := newTestManager()

	processInputCalled := false
	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, input model.UserInput, _ string) (*StateResult, error) {
		processInputCalled = true
		if input.TextValue() != "some answer" {
			t.Errorf("expected input text %q, got %q", "some answer", input.TextValue())
		}
		return &StateResult{
			Message:    model.NewTextMessage("next step"),
			IsComplete: false,
		}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("some answer"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !processInputCalled {
		t.Error("expected ProcessInput to be called for non-command text")
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "next step" {
		t.Errorf("expected sent message text %q, got %q", "next step", got)
	}
}

func TestOnUpdate_MattermostPlainCommand_ResolvesAndRoutes(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "start"}
	deps.plugins.ResolveCommandFn = func(input string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		if input == "start" {
			return "core", def, nil
		}
		return "", nil, nil
	}

	var started bool
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, pluginID string, commandName string, _ string) (*StateResult, error) {
		started = true
		if pluginID != "core" {
			t.Fatalf("expected pluginID %q, got %q", "core", pluginID)
		}
		if commandName != "start" {
			t.Fatalf("expected commandName %q, got %q", "start", commandName)
		}
		return &StateResult{Message: model.NewTextMessage("ok")}, nil
	}

	err := mgr.OnUpdate(context.Background(), Update{
		ChannelType:    model.ChannelMattermost,
		PlatformUserID: "user123",
		Input:          model.TextInput{Text: "start"},
		ChatID:         "chat42",
		Username:       "testuser",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !started {
		t.Fatal("expected StartCommand to be called")
	}
}

func TestOnUpdate_MattermostPlainText_RemainsInput(t *testing.T) {
	mgr, deps := newTestManager()

	deps.plugins.ResolveCommandFn = func(input string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, nil
	}

	var gotText string
	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, input model.UserInput, _ string) (*StateResult, error) {
		gotText = input.TextValue()
		return &StateResult{}, nil
	}

	err := mgr.OnUpdate(context.Background(), Update{
		ChannelType:    model.ChannelMattermost,
		PlatformUserID: "user123",
		Input:          model.TextInput{Text: "hello bot"},
		ChatID:         "chat42",
		Username:       "testuser",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotText != "hello bot" {
		t.Fatalf("expected plain text input, got %q", gotText)
	}
}

func TestOnUpdate_UnknownCommand_NoError(t *testing.T) {
	mgr, deps := newTestManager()

	deps.plugins.ResolveCommandFn = func(input string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, nil // command not found
	}

	// OnUpdate should not return an error because handleError absorbs the silent error.
	err := mgr.OnUpdate(context.Background(), makeUpdate("/nonexistent"))
	if err != nil {
		t.Fatalf("expected nil error for unknown command, got: %v", err)
	}

	// No message should be sent to the user for a silent error.
	msgs := deps.adapter.chatMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages sent for silent error, got %d", len(msgs))
	}
}

func TestOnUpdate_AccessDenied_SendsMessage(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "admin"}
	deps.plugins.ResolveCommandFn = func(input string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.authorizer.CheckCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ *model.RoleRequirements) (bool, error) {
		return false, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/admin"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 access denied message, got %d", len(msgs))
	}
	// The message comes from i18n.Get("error.access_denied", loc), which
	// without i18n initialization returns the key itself.
	got := firstTextBlock(msgs[0].msg)
	if got == "" {
		t.Error("expected non-empty access denied message")
	}
}

// ---------------------------------------------------------------------------
// Tests for handleCommand (via OnUpdate for exported access)
// ---------------------------------------------------------------------------

func TestHandleCommand_NotFound_SilentError(t *testing.T) {
	mgr, deps := newTestManager()

	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/ghost"))
	if err != nil {
		t.Fatalf("expected nil error (silent), got: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages for silent error, got %d", len(msgs))
	}
}

func TestHandleCommand_Ambiguous_SendsDisambiguation(t *testing.T) {
	mgr, deps := newTestManager()

	candidates := []model.CommandCandidate{
		{
			PluginID:    "pluginA",
			CommandName: "start",
			FQName:      "pluginA.start",
			Descriptions: map[string]string{
				"en": "Start A",
				"ru": "Запуск A",
			},
			Description: "Start A",
		},
		{PluginID: "pluginB", CommandName: "start", FQName: "pluginB.start", Description: "Start B"},
	}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, candidates
	}
	deps.userService.FindOrCreateUserFn = func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID, _ ...string) (*model.GlobalUser, error) {
		return &model.GlobalUser{ID: 1, Locale: "ru-RU"}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/start"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 disambiguation message, got %d", len(msgs))
	}
	ob := firstOptionsBlock(msgs[0].msg)
	if ob == nil {
		t.Fatal("disambiguation message should contain an OptionsBlock")
	}
	if ob.Options[0].Label != "Запуск A" {
		t.Errorf("option[0].Label = %q, want %q", ob.Options[0].Label, "Запуск A")
	}
	if ob.Options[0].Value != "/pluginA.start" {
		t.Errorf("option[0].Value = %q, want %q", ob.Options[0].Value, "/pluginA.start")
	}
	if ob.Options[1].Label != "Start B" {
		t.Errorf("option[1].Label = %q, want %q", ob.Options[1].Label, "Start B")
	}
}

func TestHandleCommand_AuthorizationDenied_SendsAccessDenied(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "secret"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.authorizer.CheckCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ *model.RoleRequirements) (bool, error) {
		return false, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/secret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 access denied message, got %d", len(msgs))
	}
}

func TestHandleCommand_ImmediateCommand_RoutesEvent(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "status"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}

	routed := false
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = true
		return &contract.EventResponse{}, nil
	}

	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "status",
			IsComplete:  true,
			Params:      model.OptionMap{},
		}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/status"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !routed {
		t.Error("expected immediate command to be routed")
	}
}

func TestHandleCommand_MultiStep_SendsStepMessage(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "deploy"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}

	stepMsg := model.NewTextMessage("Which environment?")
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "deploy",
			IsComplete:  false,
			Message:     stepMsg,
		}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/deploy"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 step message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "Which environment?" {
		t.Errorf("expected step message %q, got %q", "Which environment?", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for handleInput (via OnUpdate)
// ---------------------------------------------------------------------------

func TestHandleInput_ActiveDialog_Completes_RoutesCommand(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "deploy",
			IsComplete:  true,
			Params:      model.OptionMap{"env": "prod"},
		}, nil
	}

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, event)
		if len(routed) == 1 && event.PluginID != "pluginA" {
			t.Errorf("expected first pluginID %q, got %q", "pluginA", event.PluginID)
		}
		return &contract.EventResponse{}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("prod"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routed) == 0 {
		t.Error("expected completed dialog to route event")
	}
	if len(routed) != 1 {
		t.Fatalf("expected only command route; plugin menu is sent through state, got %d event(s)", len(routed))
	}
}

func TestHandleInput_ActiveDialog_Continues_SendsNextStep(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			Message:    model.NewTextMessage("Choose version:"),
			IsComplete: false,
		}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("staging"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 step message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "Choose version:" {
		t.Errorf("expected %q, got %q", "Choose version:", got)
	}
}

func TestHandleInput_NoActiveDialog_ReturnsError(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, state.ErrNoActiveDialog
	}

	// OnUpdate handles errors internally, so the returned error is nil, but
	// an error message is sent to the user. The error path goes through
	// handleError which treats non-AppError as unexpected -> sends generic msg.
	err := mgr.OnUpdate(context.Background(), makeUpdate("random text"))
	if err != nil {
		t.Fatalf("OnUpdate returned error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 error message for no active dialog, got %d", len(msgs))
	}
}

func TestHandleInput_FileInput_NoActiveDialog_SilentlyIgnored(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, state.ErrNoActiveDialog
	}

	update := Update{
		ChannelType:    model.ChannelTelegram,
		PlatformUserID: "user123",
		Input:          model.FileInput{Caption: "", Files: []model.FileRef{{ID: "f1"}}},
		ChatID:         "chat42",
	}

	err := mgr.OnUpdate(context.Background(), update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages for silently ignored file, got %d", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// Tests for handleError (via OnUpdate triggering various error types)
// ---------------------------------------------------------------------------

func TestHandleError_SeverityUser_SendsUserMessage(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, errs.NewUserError(errs.ErrInvalidInput, "bad input value")
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("bad"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 user error message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "bad input value" {
		t.Errorf("expected user error message %q, got %q", "bad input value", got)
	}
}

func TestHandleError_SeveritySilent_NoMessageSent(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, errs.NewSilentError(errs.ErrCommandNotFound, "ignored")
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("something"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages for silent error, got %d", len(msgs))
	}
}

func TestHandleError_SeverityInternal_SendsGenericMessage(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, errs.NewInternalError("db crashed")
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("hi"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 generic error message, got %d", len(msgs))
	}
	// Internal errors send the i18n "error.internal" key; without bundle init
	// the key itself is returned.
	got := firstTextBlock(msgs[0].msg)
	if got == "" {
		t.Error("expected non-empty generic error message")
	}
}

func TestHandleError_NonAppError_SendsGenericMessage(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, errors.New("something unexpected")
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("oops"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 generic error message, got %d", len(msgs))
	}
	got := firstTextBlock(msgs[0].msg)
	if got != "An error occurred. Please try again." {
		t.Errorf("expected generic error message, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for handleCommand directly (unexported, same package)
// ---------------------------------------------------------------------------

func TestHandleCommand_Direct_CommandNotFound(t *testing.T) {
	mgr, deps := newTestManager()

	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/nope"}, "chat1", "en", "")
	if err == nil {
		t.Fatal("expected error for command not found")
	}

	var appErr *errs.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Severity != errs.SeveritySilent {
		t.Errorf("expected SeveritySilent, got %v", appErr.Severity)
	}
}

func TestHandleCommand_Direct_Ambiguous(t *testing.T) {
	mgr, deps := newTestManager()

	candidates := []model.CommandCandidate{
		{PluginID: "a", CommandName: "cmd", FQName: "a.cmd"},
		{PluginID: "b", CommandName: "cmd", FQName: "b.cmd"},
	}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "", nil, candidates
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/cmd"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 disambiguation message, got %d", len(msgs))
	}
}

func TestHandleCommand_Direct_AuthDenied(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "restricted"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.authorizer.CheckCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ *model.RoleRequirements) (bool, error) {
		return false, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/restricted"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("handleCommand returned error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 access denied message, got %d", len(msgs))
	}
}

func TestHandleCommand_Direct_ImmediateRoutes(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "hello"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}

	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "hello",
			IsComplete:  true,
		}, nil
	}

	routed := false
	deps.router.RouteEventFn = func(_ context.Context, e contract.Event) (*contract.EventResponse, error) {
		routed = true
		return &contract.EventResponse{}, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/hello"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !routed {
		t.Error("expected event to be routed for immediate command")
	}
}

func TestHandleCommand_Direct_MultiStep(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "wizard"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}

	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "wizard",
			IsComplete:  false,
			Message:     model.NewTextMessage("Step 1: enter name"),
		}, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/wizard"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 step message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "Step 1: enter name" {
		t.Errorf("expected %q, got %q", "Step 1: enter name", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for handleInput directly (unexported, same package)
// ---------------------------------------------------------------------------

func TestHandleInput_Direct_Completes_Routes(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "greet",
			IsComplete:  true,
			Params:      model.OptionMap{"name": "Alice"},
		}, nil
	}

	routed := false
	deps.router.RouteEventFn = func(_ context.Context, e contract.Event) (*contract.EventResponse, error) {
		routed = true
		return &contract.EventResponse{}, nil
	}

	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "Alice"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !routed {
		t.Error("expected completed input to route event")
	}
}

func TestHandleInput_Direct_Continues(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			Message:    model.NewTextMessage("Next step"),
			IsComplete: false,
		}, nil
	}

	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "val"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestHandleInput_Direct_NoActiveDialog(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, state.ErrNoActiveDialog
	}

	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "hello"}, "chat1", "en", "")
	if !errors.Is(err, state.ErrNoActiveDialog) {
		t.Fatalf("expected ErrNoActiveDialog, got: %v", err)
	}
}

func TestHandleInput_Direct_FileInput_NoActiveDialog_ReturnsNil(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return nil, state.ErrNoActiveDialog
	}

	fileInput := model.FileInput{Caption: "", Files: []model.FileRef{{ID: "f1"}}}
	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, fileInput, "chat1", "en", "")
	if err != nil {
		t.Fatalf("expected nil for file input without dialog, got: %v", err)
	}
}

func TestHandleInput_Direct_EmptyPluginID_FallsBackToRegistry(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "", // empty — should fall back
			CommandName: "deploy",
			IsComplete:  true,
			Params:      model.OptionMap{},
		}, nil
	}

	deps.plugins.GetPluginIDByCommandFn = func(name string) string {
		if name == "deploy" {
			return "pluginFallback"
		}
		return ""
	}

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, e contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, e)
		return &contract.EventResponse{}, nil
	}

	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "yes"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routed) == 0 {
		t.Error("expected event to be routed")
	}
	if routed[0].PluginID != "pluginFallback" {
		t.Errorf("expected fallback pluginID %q, got %q", "pluginFallback", routed[0].PluginID)
	}
}

func TestDispatchCompletedCommand_AutoReturnsPluginMenu(t *testing.T) {
	mgr, deps := newTestManager()

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, event)
		return &contract.EventResponse{}, nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, chatID string, pluginID string, commandName string, loc string) (*StateResult, error) {
		if chatID != "chat1" || pluginID != "core" || commandName != "plugins" || loc != "en" {
			t.Fatalf("unexpected auto-return StartCommand args: chat=%q plugin=%q command=%q locale=%q", chatID, pluginID, commandName, loc)
		}
		return &StateResult{PluginID: "core", CommandName: "plugins", IsComplete: false}, nil
	}
	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, chatID string, input model.UserInput, loc string) (*StateResult, error) {
		if chatID != "chat1" || input.TextValue() != "pluginA" || loc != "en" {
			t.Fatalf("unexpected auto-return ProcessInput args: chat=%q input=%q locale=%q", chatID, input.TextValue(), loc)
		}
		return &StateResult{
			PluginID:    "core",
			CommandName: "plugins",
			Message:     model.NewTextMessage("plugin menu"),
			IsComplete:  false,
		}, nil
	}

	err := mgr.dispatchCompletedCommand(context.Background(), completedCommand{
		userID:      1,
		channelType: model.ChannelTelegram,
		chatID:      "chat1",
		pluginID:    "pluginA",
		commandName: "deploy",
		params:      model.OptionMap{"env": "prod"},
		locale:      "en",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routed) != 1 {
		t.Fatalf("expected only original command route, got %d", len(routed))
	}
	if routed[0].PluginID != "pluginA" {
		t.Fatalf("first routed pluginID = %q, want %q", routed[0].PluginID, "pluginA")
	}
	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 plugin menu message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "plugin menu" {
		t.Fatalf("plugin menu text = %q, want %q", got, "plugin menu")
	}
}

func TestDispatchCompletedCommand_SkipsPluginMenuForPreservedDialogCommand(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.IsPreservesDialogFn = func(pluginID, commandName string) bool {
		return pluginID == "core" && commandName == "resume"
	}

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, event)
		return &contract.EventResponse{}, nil
	}

	err := mgr.dispatchCompletedCommand(context.Background(), completedCommand{
		userID:      1,
		channelType: model.ChannelTelegram,
		chatID:      "chat1",
		pluginID:    "core",
		commandName: "resume",
		locale:      "en",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(routed) != 1 {
		t.Fatalf("expected only the original command to be routed, got %d event(s)", len(routed))
	}
}

func TestDispatchCompletedCommand_IgnoresPluginMenuFailure(t *testing.T) {
	mgr, deps := newTestManager()

	callCount := 0
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		callCount++
		return &contract.EventResponse{}, nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, pluginID string, commandName string, _ string) (*StateResult, error) {
		if pluginID == "core" && commandName == "plugins" {
			return nil, errors.New("menu send failed")
		}
		return &StateResult{}, nil
	}

	err := mgr.dispatchCompletedCommand(context.Background(), completedCommand{
		userID:      1,
		channelType: model.ChannelTelegram,
		chatID:      "chat1",
		pluginID:    "pluginA",
		commandName: "run",
		locale:      "en",
	})
	if err != nil {
		t.Fatalf("expected menu failure to be ignored, got: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected only original command to be routed, got %d call(s)", callCount)
	}
}

func TestOnUpdate_PluginError_ReturnsPluginMenu(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "fail"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, pluginID string, commandName string, _ string) (*StateResult, error) {
		if pluginID == "core" && commandName == "plugins" {
			return &StateResult{PluginID: "core", CommandName: "plugins", IsComplete: false}, nil
		}
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "fail",
			IsComplete:  true,
		}, nil
	}
	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, input model.UserInput, _ string) (*StateResult, error) {
		if input.TextValue() != "pluginA" {
			t.Fatalf("auto-return plugin input = %q, want %q", input.TextValue(), "pluginA")
		}
		return &StateResult{
			PluginID:    "core",
			CommandName: "plugins",
			Message:     model.NewTextMessage("plugin menu"),
			IsComplete:  false,
		}, nil
	}

	var routed []contract.Event
	deps.router.RouteEventFn = func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
		routed = append(routed, event)
		if event.PluginID == "pluginA" {
			return &contract.EventResponse{Error: "plugin crashed"}, nil
		}
		return &contract.EventResponse{}, nil
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/fail"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected error message and plugin menu, got %d", len(msgs))
	}
	if len(routed) != 1 {
		t.Fatalf("expected failed command route only, got %d event(s)", len(routed))
	}
	if routed[0].PluginID != "pluginA" {
		t.Fatalf("first routed pluginID = %q, want %q", routed[0].PluginID, "pluginA")
	}
}

// ---------------------------------------------------------------------------
// Tests for handleError directly (unexported, same package)
// ---------------------------------------------------------------------------

func TestHandleError_Direct_SeverityUser(t *testing.T) {
	mgr, deps := newTestManager()

	appErr := errs.NewUserError(errs.ErrInvalidInput, "wrong format")
	mgr.handleError(context.Background(), model.ChannelTelegram, "chat1", 1, appErr)

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if got := firstTextBlock(msgs[0].msg); got != "wrong format" {
		t.Errorf("expected %q, got %q", "wrong format", got)
	}
}

func TestHandleError_Direct_SeveritySilent(t *testing.T) {
	mgr, deps := newTestManager()

	appErr := errs.NewSilentError(errs.ErrCommandNotFound, "no such command")
	mgr.handleError(context.Background(), model.ChannelTelegram, "chat1", 1, appErr)

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 0 {
		t.Errorf("expected no messages for silent error, got %d", len(msgs))
	}
}

func TestHandleError_Direct_SeverityInternal(t *testing.T) {
	mgr, deps := newTestManager()

	appErr := errs.NewInternalError("storage timeout")
	mgr.handleError(context.Background(), model.ChannelTelegram, "chat1", 1, appErr)

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for internal error, got %d", len(msgs))
	}
	// The message is i18n.Get("error.internal", ...) — returns the key without bundle.
	got := firstTextBlock(msgs[0].msg)
	if got == "" {
		t.Error("expected non-empty message for internal error")
	}
}

func TestHandleError_Direct_NonAppError(t *testing.T) {
	mgr, deps := newTestManager()

	mgr.handleError(context.Background(), model.ChannelTelegram, "chat1", 1, errors.New("boom"))

	msgs := deps.adapter.chatMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for non-AppError, got %d", len(msgs))
	}
	got := firstTextBlock(msgs[0].msg)
	if got != "An error occurred. Please try again." {
		t.Errorf("expected generic fallback message, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for focus tracking integration
// ---------------------------------------------------------------------------

func TestFocusTracker_RecordedOnImmediateCommand(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "quick"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginX", def, nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginX",
			CommandName: "quick",
			IsComplete:  true,
		}, nil
	}
	deps.router.RouteEventFn = func(_ context.Context, _ contract.Event) (*contract.EventResponse, error) {
		return &contract.EventResponse{}, nil
	}

	_ = mgr.OnUpdate(context.Background(), makeUpdate("/quick"))

	deps.focus.mu.Lock()
	defer deps.focus.mu.Unlock()
	if len(deps.focus.recorded) == 0 {
		t.Fatal("expected focus to be recorded")
	}
	if deps.focus.recorded[0] != "pluginX" {
		t.Errorf("expected recorded plugin %q, got %q", "pluginX", deps.focus.recorded[0])
	}
}

func TestFocusTracker_RecordedOnDialogComplete(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginY",
			CommandName: "flow",
			IsComplete:  true,
			Params:      model.OptionMap{},
		}, nil
	}
	deps.router.RouteEventFn = func(_ context.Context, _ contract.Event) (*contract.EventResponse, error) {
		return &contract.EventResponse{}, nil
	}

	_ = mgr.OnUpdate(context.Background(), makeUpdate("done"))

	deps.focus.mu.Lock()
	defer deps.focus.mu.Unlock()
	if len(deps.focus.recorded) == 0 {
		t.Fatal("expected focus to be recorded after dialog complete")
	}
	if deps.focus.recorded[0] != "pluginY" {
		t.Errorf("expected recorded plugin %q, got %q", "pluginY", deps.focus.recorded[0])
	}
}

// ---------------------------------------------------------------------------
// Tests for OnUpdate - FindOrCreateUser failure
// ---------------------------------------------------------------------------

func TestOnUpdate_FindOrCreateUserError_ReturnsError(t *testing.T) {
	mgr, deps := newTestManager()

	deps.userService.FindOrCreateUserFn = func(_ context.Context, _ model.ChannelType, _ model.PlatformUserID, _ ...string) (*model.GlobalUser, error) {
		return nil, errors.New("db down")
	}

	err := mgr.OnUpdate(context.Background(), makeUpdate("/start"))
	if err == nil {
		t.Fatal("expected error when FindOrCreateUser fails")
	}
	if err.Error() != "db down" {
		t.Errorf("expected %q, got %q", "db down", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Tests for CancelCommand called before StartCommand
// ---------------------------------------------------------------------------

func TestHandleCommand_CancelsExistingDialog_BeforeStart(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "new"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.state.IsPreservesDialogFn = func(_, _ string) bool { return false }

	cancelCalled := false
	deps.state.CancelCommandFn = func(_ context.Context, _ model.GlobalUserID) error {
		cancelCalled = true
		return nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "new",
			IsComplete:  false,
			Message:     model.NewTextMessage("step 1"),
		}, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/new"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cancelCalled {
		t.Error("expected CancelCommand to be called before starting new command")
	}
}

func TestHandleCommand_PreservesDialog_DoesNotCancel(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "keep", PreservesDialog: true}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.state.IsPreservesDialogFn = func(_, _ string) bool { return true }

	cancelCalled := false
	deps.state.CancelCommandFn = func(_ context.Context, _ model.GlobalUserID) error {
		cancelCalled = true
		return nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "keep",
			IsComplete:  false,
			Message:     model.NewTextMessage("step"),
		}, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/keep"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelCalled {
		t.Error("CancelCommand should not be called when PreservesDialog is true")
	}
}

// ---------------------------------------------------------------------------
// Test for handleInput with non-empty completion message
// ---------------------------------------------------------------------------

func TestHandleInput_CompletionWithMessage_SendsAndRoutes(t *testing.T) {
	mgr, deps := newTestManager()

	deps.state.ProcessInputFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ model.UserInput, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "run",
			IsComplete:  true,
			Params:      model.OptionMap{},
			Message:     model.NewTextMessage("Processing..."),
		}, nil
	}

	routed := false
	deps.router.RouteEventFn = func(_ context.Context, _ contract.Event) (*contract.EventResponse, error) {
		routed = true
		return &contract.EventResponse{}, nil
	}

	err := mgr.handleInput(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "go"}, "chat1", "en", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msgs := deps.adapter.chatMessages()
	if len(msgs) == 0 {
		t.Fatal("expected completion message before routing")
	}
	if got := firstTextBlock(msgs[0].msg); got != "Processing..." {
		t.Errorf("expected %q, got %q", "Processing...", got)
	}
	if !routed {
		t.Error("expected event to be routed after completion")
	}
}

// ---------------------------------------------------------------------------
// Test for routing error (plugin returns error string)
// ---------------------------------------------------------------------------

func TestHandleCommand_PluginReturnsError(t *testing.T) {
	mgr, deps := newTestManager()

	def := &state.CommandDefinition{Name: "fail"}
	deps.plugins.ResolveCommandFn = func(_ string) (string, *state.CommandDefinition, []model.CommandCandidate) {
		return "pluginA", def, nil
	}
	deps.state.StartCommandFn = func(_ context.Context, _ model.GlobalUserID, _ string, _ string, _ string, _ string) (*StateResult, error) {
		return &StateResult{
			PluginID:    "pluginA",
			CommandName: "fail",
			IsComplete:  true,
		}, nil
	}
	deps.router.RouteEventFn = func(_ context.Context, _ contract.Event) (*contract.EventResponse, error) {
		return &contract.EventResponse{Error: "plugin crashed"}, nil
	}

	err := mgr.handleCommand(context.Background(), 1, model.ChannelTelegram, model.TextInput{Text: "/fail"}, "chat1", "en", "")
	if err == nil {
		t.Fatal("expected error when plugin returns error response")
	}
	if !errors.Is(err, nil) { // it's a fmt.Errorf, not nil, just check it exists
		// The error message should contain the plugin error text.
		if got := err.Error(); got == "" {
			t.Error("expected non-empty error message")
		}
	}
}
