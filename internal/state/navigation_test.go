package state

import (
	"context"
	"encoding/json"
	"testing"

	"SuperBotGo/internal/model"
)

func TestBackPreservesChoicesAndPagination(t *testing.T) {
	cmd := &CommandDefinition{Name: "choose", AllowBack: true, Nodes: []CommandNode{
		StepNode{ParamName: "choice", MessageBuilder: msgBuilder("choose"), Pagination: &PaginationConfig{
			PageProvider: func(_ StepContext, page int) OptionsPage {
				return OptionsPage{Options: []model.Option{{Label: "Choice", Value: "choice"}}, HasMore: page == 0}
			},
		}},
	}}
	h := NewDslStateHandler(cmd)
	s, _ := h.CreateNewState(cmd.Name)
	for page := 0; page < 2; page++ {
		msg := h.BuildStepMessage(context.Background(), 1, s, "en")
		opts := msg.Blocks[len(msg.Blocks)-1].(model.OptionsBlock).Options
		if len(opts) != 3 || opts[0].Value != "choice" || opts[2].Value != dialogBackPrefix+s.(*DslState).NavigationToken {
			t.Fatalf("choices lost on page %d: %+v", page, opts)
		}
		s, _, _ = h.ProcessInput(context.Background(), 1, s, model.CallbackInput{Data: PageNext}, "en")
		if len(s.(*DslState).History) != 0 {
			t.Fatal("pagination added a dialog step")
		}
	}
}

func TestBackRemainsVisibleInLongLists(t *testing.T) {
	opts := make([]model.Option, 25)
	for i := range opts {
		opts[i] = model.Option{Value: "choice"}
	}
	msg := model.Message{Blocks: []model.ContentBlock{model.OptionsBlock{Options: opts}}}
	result := appendBackButton(msg, model.Option{Value: "back"})
	got := result.Blocks[0].(model.OptionsBlock).Options
	if len(got) != 26 || got[0].Value != "back" || opts[0].Value != "choice" {
		t.Fatal("navigation missing or original options modified")
	}
}

func TestBackRestoresBranchAndSurvivesPersistence(t *testing.T) {
	cmd := &CommandDefinition{Name: "tasks", AllowBack: true, Nodes: []CommandNode{
		StepNode{ParamName: "doc", MessageBuilder: msgBuilder("document")},
		BranchNode{OnParam: "doc", Cases: map[string][]CommandNode{
			"diary": {StepNode{ParamName: "detail", MessageBuilder: msgBuilder("detail")}},
		}},
		StepNode{ParamName: "hidden", MessageBuilder: msgBuilder("hidden"), Condition: func(model.OptionMap) bool { return false }},
		StepNode{ParamName: "upload", MessageBuilder: msgBuilder("file")},
	}}
	h := NewDslStateHandler(cmd)
	s, _ := h.CreateNewState(cmd.Name)
	ctx := context.Background()
	s, _, _ = h.ProcessInput(ctx, 1, s, model.TextInput{Text: "diary"}, "en")
	s, _, _ = h.ProcessInput(ctx, 1, s, model.TextInput{Text: "old detail"}, "en")
	// Exercise the JSON round trip used by persistent dialog storage.
	data, _ := json.Marshal(h.PersistState(s))
	var persisted model.DialogState
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	s, _ = h.RestoreState(persisted)
	back := model.CallbackInput{Data: dialogBackPrefix + s.(*DslState).NavigationToken}
	s, out, err := h.ProcessInput(ctx, 1, s, back, "en")
	if err != nil || out.IsComplete || out.IsCancelled {
		t.Fatalf("back: %+v %v", out, err)
	}
	ds := s.(*DslState)
	if ds.Params["doc"] != "diary" || len(ds.Params) != 1 {
		t.Fatalf("params after back: %v", ds.Params)
	}
	if cmd.CurrentStep(StepContext{Params: ds.Params}).ParamName != "detail" {
		t.Fatal("did not return to the visible step")
	}
	// A repeated click on the old button cannot rewind a second time.
	s, _, _ = h.ProcessInput(ctx, 1, s, back, "en")
	if len(ds.Params) != 1 {
		t.Fatal("stale button changed answers")
	}
	s, _, _ = h.ProcessInput(ctx, 1, s, model.CallbackInput{Data: dialogBackPrefix + ds.NavigationToken}, "en")
	s, _, _ = h.ProcessInput(ctx, 1, s, model.TextInput{Text: "report"}, "en")
	ds = s.(*DslState)
	if _, exists := ds.Params["detail"]; exists {
		t.Fatal("old branch answer survived")
	}
	if cmd.CurrentStep(StepContext{Params: ds.Params}).ParamName != "upload" {
		t.Fatal("wrong branch")
	}
	s, out, err = h.ProcessInput(ctx, 1, s, model.FileInput{Files: []model.FileRef{{ID: "csv-file"}}}, "en")
	if err != nil || !out.IsComplete {
		t.Fatalf("file did not complete: %+v %v", out, err)
	}
	if msg := h.BuildStepMessage(ctx, 1, s, "en"); len(msg.Blocks) != 0 {
		t.Fatal("completed command has a back button")
	}
}

func TestBackFirstStepCancelsAndInvalidInputDoesNotAddHistory(t *testing.T) {
	cmd := newTestCommand()
	cmd.AllowBack = true
	h := NewDslStateHandler(cmd)
	s, _ := h.CreateNewState(cmd.Name)
	s, _, _ = h.ProcessInput(context.Background(), 1, s, model.TextInput{}, "en")
	ds := s.(*DslState)
	if len(ds.History) != 0 {
		t.Fatal("invalid answer recorded")
	}
	_, out, err := h.ProcessInput(context.Background(), 1, s, model.CallbackInput{Data: dialogBackPrefix + ds.NavigationToken}, "en")
	if err != nil || !out.IsCancelled || out.IsComplete {
		t.Fatalf("cancel: %+v %v", out, err)
	}
}

func TestBackRestoresPaginationAndIsOptIn(t *testing.T) {
	cmd := newTestCommand()
	h := NewDslStateHandler(cmd)
	s, _ := h.CreateNewState(cmd.Name)
	if len(h.BuildStepMessage(context.Background(), 1, s, "en").Blocks) != 1 {
		t.Fatal("changed an existing command")
	}
	cmd.AllowBack = true
	ds := s.(*DslState)
	ds.PageState["name"] = 3
	s, _, _ = h.ProcessInput(context.Background(), 1, s, model.TextInput{Text: "Alice"}, "en")
	ds.PageState["age"] = 2
	s, _, _ = h.ProcessInput(context.Background(), 1, s, model.CallbackInput{Data: dialogBackPrefix + ds.NavigationToken}, "en")
	if ds.PageState["name"] != 3 || len(ds.PageState) != 1 {
		t.Fatalf("pages: %v", ds.PageState)
	}
	msg := h.BuildStepMessage(context.Background(), 1, s, "en")
	options := msg.Blocks[len(msg.Blocks)-1].(model.OptionsBlock)
	if len(options.Options[0].Value) > 64 {
		t.Fatal("callback exceeds Telegram limit")
	}
}

type navigationStore struct{ value *model.DialogState }

func (s *navigationStore) Save(_ context.Context, _ model.GlobalUserID, value model.DialogState) error {
	s.value = &value
	return nil
}
func (s *navigationStore) Load(context.Context, model.GlobalUserID) (*model.DialogState, error) {
	return s.value, nil
}
func (s *navigationStore) Delete(context.Context, model.GlobalUserID) error {
	s.value = nil
	return nil
}

func TestManagerBackReturnsToPluginMenuWithoutDispatch(t *testing.T) {
	store := &navigationStore{}
	m := NewManager(store)
	m.RegisterCommand("practice", &CommandDefinition{Name: "tasks", AllowBack: true, Nodes: []CommandNode{
		StepNode{ParamName: "doc", MessageBuilder: msgBuilder("doc")},
	}})
	m.RegisterCommand("core", &CommandDefinition{Name: "plugins", Nodes: []CommandNode{
		StepNode{ParamName: "plugin", MessageBuilder: msgBuilder("plugin")},
		StepNode{ParamName: "command", MessageBuilder: msgBuilder("menu")},
	}})
	ctx := context.Background()
	if _, err := m.StartCommand(ctx, 1, "chat", "practice", "tasks", "en"); err != nil {
		t.Fatal(err)
	}
	msg, request, err := m.ProcessInput(ctx, 1, "chat", model.CallbackInput{Data: dialogBackPrefix + store.value.NavigationToken}, "en")
	if err != nil || request != nil || len(msg.Blocks) == 0 {
		t.Fatalf("unexpected dispatch: %v %v", request, err)
	}
	if store.value.PluginID != "core" || store.value.Params["plugin"] != "practice" {
		t.Fatalf("wrong menu: %+v", store.value)
	}
}
