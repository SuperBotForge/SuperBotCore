package vk

import (
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"
	"context"
	"encoding/json"
	"fmt"
	vkapi "github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/events"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSilentCallback(t *testing.T) {
	acknowledged := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = r.ParseForm()
		if r.URL.Path != "/messages.sendMessageEventAnswer" || r.Form.Get("event_id") != "event1" || r.Form.Get("user_id") != "12" || r.Form.Get("peer_id") != "34" {
			t.Errorf("unexpected acknowledgment: %s %v", r.URL.Path, r.Form)
		}
		acknowledged = true
		fmt.Fprint(w, `{"response":1}`)
	}))
	defer s.Close()
	v := vkapi.NewVK("test")
	v.MethodURL = s.URL + "/"
	v.Client = s.Client()
	calls := 0
	b := &Bot{vk: v, logger: slog.Default(), handler: func(ctx context.Context, u channel.Update) error {
		calls++
		input, ok := u.Input.(model.CallbackInput)
		if !acknowledged || !ok || input.Data != "tasks" || u.ChatID != "34" || u.PlatformUserID != "12" || u.PlatformUpdateID != "vk:button:34:12:event1" {
			t.Errorf("update: %+v", u)
		}
		return nil
	}}
	b.handleMessageEvent(context.Background(), events.MessageEventObject{UserID: 12, PeerID: 34, EventID: "event1", Payload: json.RawMessage(`{"sb":"tasks"}`)})
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	k := buildKeyboard(&model.OptionsBlock{Options: []model.Option{{Label: "Tasks", Value: "tasks"}}})
	raw, _ := json.Marshal(k)
	if !strings.Contains(string(raw), `"type":"callback"`) || strings.Contains(string(raw), `"type":"text"`) {
		t.Fatal(string(raw))
	}
}

type retryFileStore struct{ filestore.FileStore }

func (retryFileStore) Get(context.Context, string) (io.ReadCloser, *filestore.FileMeta, error) {
	return io.NopCloser(strings.NewReader("task,start,end\nexample,1,2\n")), &filestore.FileMeta{Name: "tasks.csv"}, nil
}

func TestDocumentUploadRetry(t *testing.T) {
	for _, tc := range []struct {
		name, uploadError      string
		failures, wantAttempts int
		success                bool
	}{
		{"recovers", "no_free_space/var/www/pi", 1, 2, true},
		{"bounded", "no_free_space/var/www/pi", 10, 3, false},
		{"permanent", "invalid_file", 10, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uploads, servers, saves := 0, 0, 0
			var s *httptest.Server
			s = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/docs.getMessagesUploadServer":
					servers++
					fmt.Fprintf(w, `{"response":{"upload_url":%q}}`, s.URL+"/upload")
				case "/upload":
					uploads++
					f, _, err := r.FormFile("file")
					if err != nil {
						t.Error(err)
						return
					}
					defer f.Close()
					data, _ := io.ReadAll(f)
					if string(data) != "task,start,end\nexample,1,2\n" {
						t.Errorf("reader not reset: %q", data)
					}
					if uploads <= tc.failures {
						fmt.Fprintf(w, `{"error":%q}`, tc.uploadError)
					} else {
						fmt.Fprint(w, `{"file":"uploaded"}`)
					}
				case "/docs.save":
					saves++
					fmt.Fprint(w, `{"response":{"type":"doc","doc":{"id":9,"owner_id":12}}}`)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer s.Close()
			v := vkapi.NewVK("test")
			v.MethodURL = s.URL + "/"
			v.Client = s.Client()
			a := NewAdapter(v, nil, retryFileStore{})
			doc, err := a.uploadDocument(context.Background(), 12, model.FileRef{ID: "file"}, model.FileTypeDocument, "tasks.csv")
			if (err == nil) != tc.success || uploads != tc.wantAttempts || servers != uploads {
				t.Fatalf("uploads=%d servers=%d err=%v", uploads, servers, err)
			}
			if tc.success && (doc.Doc.ID != 9 || saves != 1) {
				t.Fatalf("doc=%+v saves=%d", doc, saves)
			}
			if !tc.success && saves != 0 {
				t.Fatal("saved failed upload")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = a.uploadDocument(ctx, 12, model.FileRef{ID: "file"}, model.FileTypeDocument, "tasks.csv")
			if err != context.Canceled || uploads != tc.wantAttempts {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}
