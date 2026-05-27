package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCommandPermStore struct {
	origins       []string
	pluginOrigins []string
}

func (s *fakeCommandPermStore) ListCommandSettings(context.Context, string) ([]CommandSetting, error) {
	return nil, nil
}

func (s *fakeCommandPermStore) GetCommandSetting(context.Context, string, string) (CommandSetting, bool, error) {
	return CommandSetting{}, false, nil
}

func (s *fakeCommandPermStore) GetPluginFrontendOrigins(_ context.Context, pluginID string) (PluginFrontendOrigins, bool, error) {
	if s.pluginOrigins == nil {
		return PluginFrontendOrigins{}, false, nil
	}
	return PluginFrontendOrigins{
		PluginID:       pluginID,
		AllowedOrigins: s.pluginOrigins,
	}, true, nil
}

func (s *fakeCommandPermStore) SetCommandEnabled(context.Context, string, string, bool) error {
	return nil
}

func (s *fakeCommandPermStore) SetTriggerAccess(context.Context, string, string, bool, bool) error {
	return nil
}

func (s *fakeCommandPermStore) SetPolicyExpression(context.Context, string, string, string) error {
	return nil
}

func (s *fakeCommandPermStore) GetPolicyExpression(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *fakeCommandPermStore) SetAllowedOrigins(_ context.Context, _ string, _ string, origins []string) error {
	s.origins = origins
	return nil
}

func (s *fakeCommandPermStore) SetPluginFrontendOrigins(_ context.Context, _ string, origins []string) error {
	s.pluginOrigins = origins
	return nil
}

func (s *fakeCommandPermStore) DeleteCommandSettings(context.Context, string, []string) error {
	return nil
}

func (s *fakeCommandPermStore) DeleteAllPluginCommandSettings(context.Context, string) error {
	return nil
}

func TestCommandPermHandlerSetAllowedOriginsCanonicalizesInput(t *testing.T) {
	t.Parallel()

	store := &fakeCommandPermStore{}
	mux := http.NewServeMux()
	NewCommandPermHandler(store).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/plugins/schedule/commands/http/origins",
		strings.NewReader(`{"allowed_origins":[" HTTP://LOCALHOST:5173 ","http://localhost:5173/"]}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if len(store.origins) != 1 || store.origins[0] != "http://localhost:5173" {
		t.Fatalf("origins = %#v, want [http://localhost:5173]", store.origins)
	}
}

func TestCommandPermHandlerSetAllowedOriginsRejectsPath(t *testing.T) {
	t.Parallel()

	store := &fakeCommandPermStore{}
	mux := http.NewServeMux()
	NewCommandPermHandler(store).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/plugins/schedule/commands/http/origins",
		strings.NewReader(`{"allowed_origins":["http://localhost:5173/admin"]}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if len(store.origins) != 0 {
		t.Fatalf("origins = %#v, want empty", store.origins)
	}
}

func TestCommandPermHandlerSetPluginFrontendOriginsCanonicalizesInput(t *testing.T) {
	t.Parallel()

	store := &fakeCommandPermStore{}
	mux := http.NewServeMux()
	NewCommandPermHandler(store).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/admin/plugins/schedule/frontend-origins",
		strings.NewReader(`{"allowed_origins":[" HTTP://LOCALHOST:5173 ","http://localhost:5173/"]}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if len(store.pluginOrigins) != 1 || store.pluginOrigins[0] != "http://localhost:5173" {
		t.Fatalf("plugin origins = %#v, want [http://localhost:5173]", store.pluginOrigins)
	}
}

func TestCommandPermHandlerGetPluginFrontendOriginsReturnsEmptyDefault(t *testing.T) {
	t.Parallel()

	store := &fakeCommandPermStore{}
	mux := http.NewServeMux()
	NewCommandPermHandler(store).RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/plugins/schedule/frontend-origins", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	var body PluginFrontendOrigins
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.PluginID != "schedule" || len(body.AllowedOrigins) != 0 {
		t.Fatalf("body = %#v, want empty plugin frontend origins", body)
	}
}
