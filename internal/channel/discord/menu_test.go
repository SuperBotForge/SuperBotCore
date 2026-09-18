package discord

import (
	"SuperBotGo/internal/model"
	"context"
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type menuTransport func(*http.Request) (*http.Response, error)

func (f menuTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMenuKeepsDiscordStringID(t *testing.T) {
	s, _ := discordgo.New("Bot test")
	const id = "1234567890123456789"
	var calls []string
	s.Client = &http.Client{Transport: menuTransport(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.Method)
		if r.Method != "POST" && !strings.HasSuffix(r.URL.Path, "/"+id) {
			t.Errorf("wrong message path: %s", r.URL.Path)
		}
		if r.Method == "PATCH" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["content"] != "next" || len(body["components"].([]any)) != 1 {
				t.Errorf("edit body: %v", body)
			}
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"` + id + `","channel_id":"chat"}`))}, nil
	})}
	a := NewAdapter(s, &atomic.Bool{}, nil)
	ctx := context.Background()
	got, err := a.SendToChatWithID(ctx, "chat", model.NewTextMessage("menu"))
	if err != nil || got != id {
		t.Fatalf("id %s err %v", got, err)
	}
	msg := model.Message{Blocks: []model.ContentBlock{model.TextBlock{Text: "next"}, model.OptionsBlock{Options: []model.Option{{Label: "Back", Value: "back"}}}}}
	if err := a.EditMessage(ctx, "chat", id, msg); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteMessage(ctx, "chat", id); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "POST,PATCH,DELETE" {
		t.Fatal(calls)
	}
}
