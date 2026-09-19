package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"SuperBotGo/internal/model"
	tele "gopkg.in/telebot.v3"
)

func TestMenuCallbackPayloadOnSendAndEdit(t *testing.T) {
	back := "__dialog_back:" + strings.Repeat("a", 32)
	values := []string{"show", "report", back, strings.Repeat("x", 64)}
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		raw := body["reply_markup"]
		var encoded string
		if json.Unmarshal(raw, &encoded) == nil {
			raw = []byte(encoded)
		}
		var markup struct {
			Rows [][]struct {
				Data string `json:"callback_data"`
			} `json:"inline_keyboard"`
		}
		if err := json.Unmarshal(raw, &markup); err != nil {
			t.Error(err)
		}
		if len(markup.Rows) != len(values) {
			t.Errorf("keyboard: %s", raw)
		}
		for i, row := range markup.Rows {
			if i >= len(values) {
				break
			}
			if len(row) != 1 || row[0].Data != values[i] || len(row[0].Data) > 64 {
				t.Errorf("callback changed: %+v", row)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42,"chat":{"id":123,"type":"private"},"text":"menu"}}`))
	}))
	defer server.Close()
	bot, err := tele.NewBot(tele.Settings{Token: "test", URL: server.URL, Offline: true, Client: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	a := NewAdapter(bot, &atomic.Bool{}, nil)
	options := make([]model.Option, len(values))
	for i, value := range values {
		options[i] = model.Option{Label: "Button", Value: value}
	}
	msg := model.Message{Blocks: []model.ContentBlock{model.TextBlock{Text: "menu"}, model.OptionsBlock{Options: options}}}
	if _, err := a.SendToChatWithID(context.Background(), "123", msg); err != nil {
		t.Fatal(err)
	}
	if err := a.EditMessage(context.Background(), "123", "42", msg); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !strings.HasSuffix(calls[0], "/sendMessage") || !strings.HasSuffix(calls[1], "/editMessageText") {
		t.Fatal(calls)
	}
}

func TestRawMenuCallbackReachesHandler(t *testing.T) {
	bot, err := tele.NewBot(tele.Settings{Token: "test", Offline: true, Synchronous: true})
	if err != nil {
		t.Fatal(err)
	}
	want := "__dialog_back:" + strings.Repeat("b", 32)
	var got string
	bot.Handle(tele.OnCallback, func(c tele.Context) error { got = c.Callback().Data; return nil })
	bot.ProcessUpdate(tele.Update{Callback: &tele.Callback{Data: want}})
	if got != want {
		t.Fatalf("callback = %q, want %q", got, want)
	}
}
