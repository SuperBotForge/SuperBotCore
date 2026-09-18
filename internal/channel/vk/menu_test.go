package vk

import (
	"SuperBotGo/internal/model"
	"context"
	vkapi "github.com/SevereCloud/vksdk/v3/api"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMenuSendEditDelete(t *testing.T) {
	seen := map[string]bool{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		seen[r.URL.Path] = true
		if r.Form.Get("peer_id") != "123" {
			t.Errorf("peer: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/messages.send":
			_, _ = w.Write([]byte(`{"response":42}`))
		case "/messages.edit":
			if r.Form.Get("message_id") != "42" || r.Form.Get("message") != "next" {
				t.Errorf("edit: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"response":1}`))
		case "/messages.delete":
			if r.Form.Get("message_ids") != "42" || r.Form.Get("delete_for_all") != "1" {
				t.Errorf("delete: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"response":[{"peer_id":123,"message_id":42,"response":1}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	v := vkapi.NewVK("test")
	v.MethodURL = s.URL + "/"
	v.Client = s.Client()
	a := NewAdapter(v, &atomic.Bool{}, nil)
	ctx := context.Background()
	id, err := a.SendToChatWithID(ctx, "123", model.NewTextMessage("menu"))
	if err != nil || id != "42" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	if err := a.EditMessage(ctx, "123", id, model.NewTextMessage("next")); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteMessage(ctx, "123", id); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 {
		t.Fatal(seen)
	}
}

func TestMenuKeyboardIsInlineAndFitsRows(t *testing.T) {
	opts := make([]model.Option, 10)
	for i := range opts {
		opts[i] = model.Option{Label: "Choice", Value: "value"}
	}
	k := buildKeyboard(&model.OptionsBlock{Options: opts})
	if !bool(k.Inline) || len(k.Buttons) != 5 {
		t.Fatalf("keyboard: %+v", k)
	}
	for _, row := range k.Buttons {
		if len(row) != 2 {
			t.Fatal("missing button")
		}
	}
}
