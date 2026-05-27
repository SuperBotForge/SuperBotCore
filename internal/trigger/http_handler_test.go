package trigger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type httpTestPlugin struct {
	id            string
	handleEventFn func(ctx context.Context, event contract.Event) (*contract.EventResponse, error)
}

func (p *httpTestPlugin) ID() string      { return p.id }
func (p *httpTestPlugin) Name() string    { return p.id }
func (p *httpTestPlugin) Version() string { return "1.0.0" }
func (p *httpTestPlugin) Commands() []*state.CommandDefinition {
	return nil
}
func (p *httpTestPlugin) HandleEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error) {
	return p.handleEventFn(ctx, event)
}

func TestHTTPTriggerServeHTTP_Success(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTriggers("demo", []wasmrt.TriggerDef{{
		Type:    "http",
		Name:    "incoming",
		Path:    "incoming",
		Methods: []string{http.MethodPost},
	}})

	manager := plugin.NewManager()
	manager.Register(&httpTestPlugin{
		id: "demo",
		handleEventFn: func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
			data, err := event.HTTP()
			if err != nil {
				t.Fatalf("event.HTTP() error = %v", err)
			}
			if data.Method != http.MethodPost {
				t.Fatalf("data.Method = %q, want %q", data.Method, http.MethodPost)
			}
			if data.Path != "/incoming" {
				t.Fatalf("data.Path = %q, want %q", data.Path, "/incoming")
			}
			if got := data.Query["name"]; got != "alice" {
				t.Fatalf("data.Query[name] = %q, want %q", got, "alice")
			}
			if got := data.Headers["X-Test"]; got != "123" {
				t.Fatalf("data.Headers[X-Test] = %q, want %q", got, "123")
			}
			if data.Auth == nil || data.Auth.Kind != contract.HTTPAuthUser || data.Auth.UserID != 42 {
				t.Fatalf("unexpected auth data: %#v", data.Auth)
			}

			payload, _ := json.Marshal(contract.HTTPResponseData{
				StatusCode: http.StatusCreated,
				Headers: map[string]string{
					"Content-Type": "text/plain",
					"X-Reply":      "yes",
				},
				Body: "created",
			})
			return &contract.EventResponse{Data: payload}, nil
		},
	})

	handler := NewHTTPTriggerHandler(NewRouter(registry, manager), registry)
	handler.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})

	req := httptest.NewRequest(http.MethodPost, "/api/triggers/http/demo/incoming?name=alice", strings.NewReader(`{"ok":true}`))
	req.Header.Set("X-Test", "123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusCreated; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "text/plain"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("X-Reply"), "yes"; got != want {
		t.Fatalf("X-Reply = %q, want %q", got, want)
	}
	if got, want := rec.Body.String(), "created"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHTTPTriggerServeHTTP_InvalidPluginResponse(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTriggers("demo", []wasmrt.TriggerDef{{
		Type:    "http",
		Name:    "incoming",
		Path:    "incoming",
		Methods: []string{http.MethodGet},
	}})

	manager := plugin.NewManager()
	manager.Register(&httpTestPlugin{
		id: "demo",
		handleEventFn: func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
			return &contract.EventResponse{Data: []byte("not-json")}, nil
		},
	})

	handler := NewHTTPTriggerHandler(NewRouter(registry, manager), registry)
	handler.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/triggers/http/demo/incoming", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if body := rec.Body.String(); !strings.Contains(body, "internal error") {
		t.Fatalf("body = %q, want internal error", body)
	}
}

func TestHTTPTriggerServeHTTP_PublicEndpointUsesAnonymousPrincipal(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTriggers("demo", []wasmrt.TriggerDef{{
		Type:    "http",
		Name:    "public",
		Path:    "public",
		Methods: []string{http.MethodGet},
	}})

	manager := plugin.NewManager()
	manager.Register(&httpTestPlugin{
		id: "demo",
		handleEventFn: func(_ context.Context, event contract.Event) (*contract.EventResponse, error) {
			data, err := event.HTTP()
			if err != nil {
				t.Fatalf("event.HTTP() error = %v", err)
			}
			if data.Auth != nil {
				t.Fatalf("expected anonymous auth data, got %#v", data.Auth)
			}

			payload, _ := json.Marshal(contract.HTTPResponseData{
				StatusCode: http.StatusOK,
				Body:       `{"ok":true}`,
			})
			return &contract.EventResponse{Data: payload}, nil
		},
	})

	handler := NewHTTPTriggerHandler(NewRouter(registry, manager), registry)
	handler.SetSettingLoader(func(_ context.Context, pluginID, triggerName string) (HTTPTriggerSetting, bool, error) {
		if pluginID != "demo" || triggerName != "public" {
			t.Fatalf("unexpected setting lookup for %s/%s", pluginID, triggerName)
		}
		return HTTPTriggerSetting{
			Enabled:          true,
			AllowUserKeys:    false,
			AllowServiceKeys: false,
		}, true, nil
	})
	handler.SetUserAuthenticator(func(_ *http.Request) (model.GlobalUserID, bool) {
		return 42, true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/triggers/http/demo/public", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), `{"ok":true}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHTTPTriggerServeHTTP_CORSPreflightAllowedOrigin(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTriggers("demo", []wasmrt.TriggerDef{{
		Type:    "http",
		Name:    "incoming",
		Path:    "incoming",
		Methods: []string{http.MethodPost},
	}})

	manager := plugin.NewManager()
	manager.Register(&httpTestPlugin{
		id: "demo",
		handleEventFn: func(context.Context, contract.Event) (*contract.EventResponse, error) {
			t.Fatal("preflight request must not be dispatched to plugin")
			return nil, nil
		},
	})

	handler := NewHTTPTriggerHandler(NewRouter(registry, manager), registry)
	handler.SetSettingLoader(func(context.Context, string, string) (HTTPTriggerSetting, bool, error) {
		return HTTPTriggerSetting{
			Enabled:        true,
			AllowedOrigins: []string{"http://localhost:5173"},
		}, true, nil
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/triggers/http/demo/incoming", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-Requested-With")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://localhost:5173"; got != want {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Credentials"), "true"; got != want {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Methods"), http.MethodPost; got != want {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Headers"), "X-Requested-With"; got != want {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %q", got, want)
	}
}

func TestHTTPTriggerServeHTTP_CORSRejectsUnregisteredOrigin(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterTriggers("demo", []wasmrt.TriggerDef{{
		Type:    "http",
		Name:    "incoming",
		Path:    "incoming",
		Methods: []string{http.MethodGet},
	}})

	manager := plugin.NewManager()
	manager.Register(&httpTestPlugin{
		id: "demo",
		handleEventFn: func(context.Context, contract.Event) (*contract.EventResponse, error) {
			t.Fatal("disallowed origin must not be dispatched to plugin")
			return nil, nil
		},
	})

	handler := NewHTTPTriggerHandler(NewRouter(registry, manager), registry)
	handler.SetSettingLoader(func(context.Context, string, string) (HTTPTriggerSetting, bool, error) {
		return HTTPTriggerSetting{
			Enabled:        true,
			AllowedOrigins: []string{"http://localhost:5173"},
		}, true, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/triggers/http/demo/incoming", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}
