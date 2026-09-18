package channel

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin/contract"
)

type menuAdapter struct {
	*mockChannelAdapter
	channel              model.ChannelType
	events               []string
	count                int
	failEdit, failDelete bool
}

func (a *menuAdapter) Type() model.ChannelType { return a.channel }
func (a *menuAdapter) SendToChatWithID(_ context.Context, chat string, msg model.Message) (string, error) {
	a.count++
	id := fmt.Sprint(a.count)
	a.events = append(a.events, "send:"+id+":"+firstTextBlock(msg))
	return id, nil
}
func (a *menuAdapter) EditMessage(_ context.Context, chat, id string, msg model.Message) error {
	a.events = append(a.events, "edit:"+id+":"+firstTextBlock(msg))
	if a.failEdit {
		return errors.New("edit failed")
	}
	return nil
}
func (a *menuAdapter) DeleteMessage(_ context.Context, chat, id string) error {
	a.events = append(a.events, "delete:"+id)
	if a.failDelete {
		return errors.New("delete failed")
	}
	return nil
}

func TestDialogMenuLifecyclePreservesReplyAndNotification(t *testing.T) {
	m, deps := newTestManager()
	a := &menuAdapter{mockChannelAdapter: deps.adapter, channel: model.ChannelTelegram}
	m.adapters.Register(a)
	ctx := context.Background()
	if err := m.showDialogMessage(ctx, a.channel, "chat", model.NewTextMessage("menu"), false); err != nil {
		t.Fatal(err)
	}
	if err := m.showDialogMessage(ctx, a.channel, "chat", model.NewTextMessage("question"), true); err != nil {
		t.Fatal(err)
	}
	// A notification is sent independently and must never be tracked as a menu.
	if err := m.adapters.SendToChat(ctx, a.channel, "chat", model.NewTextMessage("notification")); err != nil {
		t.Fatal(err)
	}
	deps.router.RouteEventFn = func(context.Context, contract.Event) (*contract.EventResponse, error) {
		a.events = append(a.events, "reply:accepted")
		return &contract.EventResponse{}, nil
	}
	deps.state.ProcessInputFn = func(context.Context, model.GlobalUserID, string, model.UserInput, string) (*StateResult, error) {
		return &StateResult{Message: model.NewTextMessage("new menu")}, nil
	}
	if err := m.dispatchCompletedCommand(ctx, completedCommand{userID: 1, channelType: a.channel, chatID: "chat", pluginID: "practice", commandName: "submit"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"send:1:menu", "edit:1:question", "delete:1", "reply:accepted", "send:2:new menu"}
	if !reflect.DeepEqual(a.events, want) {
		t.Fatalf("events %v, want %v", a.events, want)
	}
	if len(deps.adapter.chatMessages()) != 1 {
		t.Fatal("notification changed")
	}
}

func TestDialogMenusScopedByChannelAndTextMovesMenuToBottom(t *testing.T) {
	m, deps := newTestManager()
	tg := &menuAdapter{mockChannelAdapter: deps.adapter, channel: model.ChannelTelegram}
	vk := &menuAdapter{mockChannelAdapter: deps.adapter, channel: model.ChannelVK}
	m.adapters.Register(tg)
	m.adapters.Register(vk)
	ctx := context.Background()
	for _, a := range []*menuAdapter{tg, vk} {
		if err := m.showDialogMessage(ctx, a.channel, "123", model.NewTextMessage("menu"), false); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.showDialogMessage(ctx, tg.channel, "123", model.NewTextMessage("next"), false); err != nil {
		t.Fatal(err)
	}
	if len(vk.events) != 1 {
		t.Fatal("VK menu was affected by Telegram")
	}
	if !reflect.DeepEqual(tg.events, []string{"send:1:menu", "delete:1", "send:2:next"}) {
		t.Fatal(tg.events)
	}
}

func TestDialogMenuEditFailureFallsBackAndDeletionFailureClearsKeyboard(t *testing.T) {
	m, deps := newTestManager()
	a := &menuAdapter{mockChannelAdapter: deps.adapter, channel: model.ChannelTelegram, failEdit: true, failDelete: true}
	m.adapters.Register(a)
	ctx := context.Background()
	_ = m.showDialogMessage(ctx, a.channel, "chat", model.NewTextMessage("menu"), false)
	if err := m.showDialogMessage(ctx, a.channel, "chat", model.NewTextMessage("next"), true); err != nil {
		t.Fatal(err)
	}
	want := []string{"send:1:menu", "edit:1:next", "delete:1", "edit:1:", "send:2:next"}
	if !reflect.DeepEqual(a.events, want) {
		t.Fatal(a.events)
	}
}

func TestUnsupportedEditingReturnsError(t *testing.T) {
	r := NewAdapterRegistry()
	r.Register(&mockChannelAdapter{})
	if !errors.Is(r.EditMessageInChat(context.Background(), model.ChannelTelegram, "chat", "1", model.NewTextMessage("next")), ErrMessageOperationUnsupported) {
		t.Fatal("unsupported edit must trigger send fallback")
	}
}
