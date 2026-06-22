package core

import (
	"context"
	"errors"
	"testing"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type stubLister struct {
	plugins []plugin.PluginInfo
}

func (s *stubLister) ListUserPlugins(_ ...string) []plugin.PluginInfo {
	return s.plugins
}

type stubAuthChecker struct {
	allowFn func(pluginID, commandName string) bool
	errFn   func(pluginID, commandName string) error
}

func (s *stubAuthChecker) CheckCommand(_ context.Context, _ model.GlobalUserID, pluginID string, commandName string, _ *model.RoleRequirements) (bool, error) {
	if s.errFn != nil {
		if err := s.errFn(pluginID, commandName); err != nil {
			return false, err
		}
	}
	if s.allowFn != nil {
		return s.allowFn(pluginID, commandName), nil
	}
	return true, nil
}

type capturingAdapter struct {
	lastMsg model.Message
}

func (a *capturingAdapter) Type() model.ChannelType { return "test" }
func (a *capturingAdapter) SendToUser(_ context.Context, _ model.PlatformUserID, _ model.Message) error {
	return nil
}
func (a *capturingAdapter) SendToChat(_ context.Context, _ string, msg model.Message) error {
	a.lastMsg = msg
	return nil
}

func newTestPlugin(lister *stubLister, authChecker CommandAuthChecker) (*Plugin, *capturingAdapter) {
	adapter := &capturingAdapter{}
	reg := channel.NewAdapterRegistry()
	reg.Register(adapter)
	api := plugin.NewSenderAPI(reg, nil)

	return &Plugin{
		api:          api,
		pluginLister: lister,
		authChecker:  authChecker,
	}, adapter
}

func triggerData(pluginID string) *contract.MessengerTriggerData {
	return &contract.MessengerTriggerData{
		UserID:      1,
		ChannelType: "test",
		ChatID:      "chat1",
		CommandName: "plugins",
		Params:      model.OptionMap{"plugin": pluginID},
		Locale:      "en",
	}
}

func findOptionsBlock(msg model.Message) *model.OptionsBlock {
	for _, b := range msg.Blocks {
		if ob, ok := b.(model.OptionsBlock); ok {
			return &ob
		}
	}
	return nil
}

func findTextBlock(msg model.Message, style model.TextStyle) *model.TextBlock {
	for _, b := range msg.Blocks {
		if tb, ok := b.(model.TextBlock); ok && tb.Style == style {
			return &tb
		}
	}
	return nil
}

func testPluginInfo(id, name string, commandCount int) plugin.PluginInfo {
	commands := make([]plugin.PluginCommand, 0, commandCount)
	for i := 0; i < commandCount; i++ {
		commands = append(commands, plugin.PluginCommand{
			Name:        "cmd" + string(rune('a'+i)),
			Description: "Command " + string(rune('A'+i)),
		})
	}
	return plugin.PluginInfo{ID: id, Name: name, Commands: commands}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPluginsCommand_PaginatesPluginList(t *testing.T) {
	t.Parallel()

	plugins := make([]plugin.PluginInfo, 0, pluginListPageSize+1)
	for i := 0; i < pluginListPageSize+1; i++ {
		suffix := string(rune('A' + i))
		plugins = append(plugins, testPluginInfo("plugin"+suffix, "Plugin "+suffix, 1))
	}
	lister := &stubLister{plugins: plugins}
	handler := state.NewDslStateHandler(PluginsCommand(lister, nil))
	dialogState, err := handler.CreateNewState("plugins")
	if err != nil {
		t.Fatal(err)
	}

	msg := handler.BuildStepMessage(context.Background(), 1, dialogState, "en")
	ob := findOptionsBlock(msg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}
	if len(ob.Options) != pluginListPageSize+1 {
		t.Fatalf("expected %d plugin options plus next, got %d", pluginListPageSize, len(ob.Options))
	}
	if ob.Options[0].Value != "pluginA" {
		t.Fatalf("first plugin option = %q, want %q", ob.Options[0].Value, "pluginA")
	}
	if ob.Options[len(ob.Options)-1].Value != state.PageNext {
		t.Fatalf("last option = %q, want page next", ob.Options[len(ob.Options)-1].Value)
	}

	dialogState, outcome, err := handler.ProcessInput(context.Background(), 1, dialogState, model.CallbackInput{Data: state.PageNext}, "en")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.IsComplete {
		t.Fatal("pagination should not complete the command")
	}
	ob = findOptionsBlock(outcome.Message)
	if ob == nil {
		t.Fatal("expected OptionsBlock after next page")
	}
	if len(ob.Options) != 2 {
		t.Fatalf("expected remaining plugin plus previous, got %d options", len(ob.Options))
	}
	if ob.Options[0].Value != "pluginI" {
		t.Fatalf("page 2 plugin option = %q, want %q", ob.Options[0].Value, "pluginI")
	}
	if ob.Options[1].Value != state.PagePrev {
		t.Fatalf("page 2 nav option = %q, want page previous", ob.Options[1].Value)
	}

	_ = dialogState
}

func TestPluginsCommand_PaginatesPluginCommands(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		testPluginInfo("sched", "Schedule", pluginCommandPageSize+1),
	}}
	handler := state.NewDslStateHandler(PluginsCommand(lister, nil))
	dialogState, err := handler.CreateNewState("plugins")
	if err != nil {
		t.Fatal(err)
	}

	dialogState, outcome, err := handler.ProcessInput(context.Background(), 1, dialogState, model.CallbackInput{Data: "sched"}, "en")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.IsComplete {
		t.Fatal("selecting a plugin should show command menu")
	}
	ob := findOptionsBlock(outcome.Message)
	if ob == nil {
		t.Fatal("expected command OptionsBlock")
	}
	if len(ob.Options) != pluginCommandPageSize+2 {
		t.Fatalf("expected commands, back, and next; got %d options", len(ob.Options))
	}
	if ob.Options[0].Value != "/sched.cmda" {
		t.Fatalf("first command value = %q, want %q", ob.Options[0].Value, "/sched.cmda")
	}
	if ob.Options[pluginCommandPageSize].Value != "/plugins" {
		t.Fatalf("back option value = %q, want /plugins", ob.Options[pluginCommandPageSize].Value)
	}
	if ob.Options[len(ob.Options)-1].Value != state.PageNext {
		t.Fatalf("last option = %q, want page next", ob.Options[len(ob.Options)-1].Value)
	}

	dialogState, outcome, err = handler.ProcessInput(context.Background(), 1, dialogState, model.CallbackInput{Data: state.PageNext}, "en")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.IsComplete {
		t.Fatal("command pagination should not complete the command")
	}
	ob = findOptionsBlock(outcome.Message)
	if ob == nil {
		t.Fatal("expected command OptionsBlock after next page")
	}
	if len(ob.Options) != 3 {
		t.Fatalf("expected remaining command, back, and previous; got %d options", len(ob.Options))
	}
	if ob.Options[0].Value != "/sched.cmdh" {
		t.Fatalf("remaining command value = %q, want %q", ob.Options[0].Value, "/sched.cmdh")
	}
	if ob.Options[1].Value != "/plugins" {
		t.Fatalf("back option value = %q, want /plugins", ob.Options[1].Value)
	}
	if ob.Options[2].Value != state.PagePrev {
		t.Fatalf("previous option value = %q, want page previous", ob.Options[2].Value)
	}
}

func TestHandlePlugins_ShowsLocalizedCommandTextAndRoutesByFQName(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "sched", Name: "Schedule", Commands: []plugin.PluginCommand{
			{
				Name: "view",
				Descriptions: map[string]string{
					"en": "View schedule",
					"ru": "Посмотреть расписание",
				},
				Description: "View schedule",
			},
			{Name: "find", Description: "Search"},
		}},
	}}
	p, adapter := newTestPlugin(lister, nil)

	data := triggerData("sched")
	data.Locale = "ru-RU"
	err := p.handlePlugins(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock in response")
	}

	// 2 commands + 1 back button
	if len(ob.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(ob.Options))
	}
	if ob.Options[0].Value != "/sched.view" {
		t.Errorf("option[0].Value = %q, want %q", ob.Options[0].Value, "/sched.view")
	}
	if ob.Options[1].Value != "/sched.find" {
		t.Errorf("option[1].Value = %q, want %q", ob.Options[1].Value, "/sched.find")
	}
	if ob.Options[0].Label != "Посмотреть расписание" {
		t.Errorf("option[0].Label = %q, want %q", ob.Options[0].Label, "Посмотреть расписание")
	}
	if ob.Options[1].Label != "Search" {
		t.Errorf("option[1].Label = %q, want %q", ob.Options[1].Label, "Search")
	}
}

func TestHandlePlugins_HiddenCommandsFiltered(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "core", Name: "Core", Commands: []plugin.PluginCommand{
			{Name: "start", Description: "Welcome"},
			{Name: "plugins", Description: "Browse"},
			{Name: "settings", Description: "Configure"},
			{Name: "link", Description: "Link accounts"},
		}},
	}}
	p, adapter := newTestPlugin(lister, nil)

	err := p.handlePlugins(context.Background(), triggerData("core"))
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}

	// settings + link + back = 3
	if len(ob.Options) != 3 {
		t.Fatalf("expected 3 options (settings, link, back), got %d", len(ob.Options))
	}

	for _, opt := range ob.Options {
		if opt.Value == "/core.start" || opt.Value == "/core.plugins" {
			t.Errorf("hidden command should not appear: %s", opt.Value)
		}
	}
}

func TestHandlePlugins_BackButton(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "x", Name: "X", Commands: []plugin.PluginCommand{
			{Name: "cmd1"},
		}},
	}}
	p, adapter := newTestPlugin(lister, nil)

	err := p.handlePlugins(context.Background(), triggerData("x"))
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}

	last := ob.Options[len(ob.Options)-1]
	if last.Value != "/plugins" {
		t.Errorf("last option value = %q, want %q", last.Value, "/plugins")
	}
}

func TestHandlePlugins_AuthFiltersCommands(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "admin", Name: "Admin", Commands: []plugin.PluginCommand{
			{Name: "public", Description: "Public cmd"},
			{Name: "secret", Description: "Admin only", Requirements: &model.RoleRequirements{SystemRole: "admin"}},
		}},
	}}
	auth := &stubAuthChecker{
		allowFn: func(_, commandName string) bool {
			return commandName != "secret"
		},
	}
	p, adapter := newTestPlugin(lister, auth)

	err := p.handlePlugins(context.Background(), triggerData("admin"))
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}

	// public + back = 2
	if len(ob.Options) != 2 {
		t.Fatalf("expected 2 options (public + back), got %d", len(ob.Options))
	}
	if ob.Options[0].Value != "/admin.public" {
		t.Errorf("option[0].Value = %q, want %q", ob.Options[0].Value, "/admin.public")
	}
}

func TestHandlePlugins_AllCommandsFiltered_ShowsNoCommands(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "restricted", Name: "Restricted", Commands: []plugin.PluginCommand{
			{Name: "cmd1"},
			{Name: "cmd2"},
		}},
	}}
	auth := &stubAuthChecker{
		allowFn: func(_, _ string) bool { return false },
	}
	p, adapter := newTestPlugin(lister, auth)

	err := p.handlePlugins(context.Background(), triggerData("restricted"))
	if err != nil {
		t.Fatal(err)
	}

	// Should show header + "no commands" text + back button
	tb := findTextBlock(adapter.lastMsg, model.StylePlain)
	if tb == nil {
		t.Fatal("expected plain text block with no-commands message")
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock with back button")
	}
	if len(ob.Options) != 1 {
		t.Fatalf("expected 1 option (back), got %d", len(ob.Options))
	}
	if ob.Options[0].Value != "/plugins" {
		t.Errorf("back button value = %q, want %q", ob.Options[0].Value, "/plugins")
	}
}

func TestHandlePlugins_AuthError_HidesCommand(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "p", Name: "P", Commands: []plugin.PluginCommand{
			{Name: "ok"},
			{Name: "err_cmd"},
		}},
	}}
	auth := &stubAuthChecker{
		allowFn: func(_, _ string) bool { return true },
		errFn: func(_, commandName string) error {
			if commandName == "err_cmd" {
				return errors.New("spicedb unavailable")
			}
			return nil
		},
	}
	p, adapter := newTestPlugin(lister, auth)

	err := p.handlePlugins(context.Background(), triggerData("p"))
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}

	// ok + back = 2 (err_cmd hidden due to error)
	if len(ob.Options) != 2 {
		t.Fatalf("expected 2 options (ok + back), got %d", len(ob.Options))
	}
}

func TestHandlePlugins_NilAuthChecker_AllowsAll(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{
		{ID: "p", Name: "P", Commands: []plugin.PluginCommand{
			{Name: "a"},
			{Name: "b"},
		}},
	}}
	p, adapter := newTestPlugin(lister, nil)

	err := p.handlePlugins(context.Background(), triggerData("p"))
	if err != nil {
		t.Fatal(err)
	}

	ob := findOptionsBlock(adapter.lastMsg)
	if ob == nil {
		t.Fatal("expected OptionsBlock")
	}

	// a + b + back = 3
	if len(ob.Options) != 3 {
		t.Fatalf("expected 3 options, got %d", len(ob.Options))
	}
}

func TestHandlePlugins_PluginNotFound(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{}}
	p, adapter := newTestPlugin(lister, nil)

	err := p.handlePlugins(context.Background(), triggerData("nonexistent"))
	if err != nil {
		t.Fatal(err)
	}

	// Should get a plain text message (not_found)
	if len(adapter.lastMsg.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adapter.lastMsg.Blocks))
	}
	tb, ok := adapter.lastMsg.Blocks[0].(model.TextBlock)
	if !ok {
		t.Fatal("expected TextBlock")
	}
	if tb.Text == "" {
		t.Error("expected non-empty not-found message")
	}
}

func TestHandlePlugins_EmptyPluginParam(t *testing.T) {
	t.Parallel()

	lister := &stubLister{plugins: []plugin.PluginInfo{}}
	p, adapter := newTestPlugin(lister, nil)

	m := &contract.MessengerTriggerData{
		UserID:      1,
		ChannelType: "test",
		ChatID:      "chat1",
		Params:      model.OptionMap{},
		Locale:      "en",
	}

	err := p.handlePlugins(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}

	if len(adapter.lastMsg.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(adapter.lastMsg.Blocks))
	}
}

func TestCountVisibleCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		commands []plugin.PluginCommand
		want     int
	}{
		{
			name: "all visible",
			commands: []plugin.PluginCommand{
				{Name: "settings"}, {Name: "link"},
			},
			want: 2,
		},
		{
			name: "hidden filtered",
			commands: []plugin.PluginCommand{
				{Name: "start"}, {Name: "plugins"}, {Name: "settings"},
			},
			want: 1,
		},
		{
			name:     "all hidden",
			commands: []plugin.PluginCommand{{Name: "start"}, {Name: "plugins"}},
			want:     0,
		},
		{
			name:     "empty",
			commands: nil,
			want:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := countVisibleCommands(plugin.PluginInfo{Commands: tc.commands})
			if got != tc.want {
				t.Errorf("countVisibleCommands() = %d, want %d", got, tc.want)
			}
		})
	}
}
