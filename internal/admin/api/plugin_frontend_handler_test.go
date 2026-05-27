package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"SuperBotGo/internal/plugin"
)

type memoryBlobStore struct {
	data map[string][]byte
}

func (s *memoryBlobStore) Put(_ context.Context, key string, data io.Reader, _ int64) error {
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	raw, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	s.data[key] = raw
	return nil
}

func (s *memoryBlobStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	raw, ok := s.data[key]
	if !ok {
		return nil, errNotFound(key)
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

func (s *memoryBlobStore) Delete(_ context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func (s *memoryBlobStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.data[key]
	return ok, nil
}

type frontendPluginStore struct {
	testPluginStore
	frontends map[string]PluginFrontendRecord
}

func (s *frontendPluginStore) SavePluginFrontend(_ context.Context, record PluginFrontendRecord) error {
	if s.frontends == nil {
		s.frontends = make(map[string]PluginFrontendRecord)
	}
	s.frontends[record.PluginID] = record
	return nil
}

func (s *frontendPluginStore) GetPluginFrontend(_ context.Context, pluginID string) (PluginFrontendRecord, error) {
	record, ok := s.frontends[pluginID]
	if !ok {
		return PluginFrontendRecord{}, errNotFound(pluginID)
	}
	return record, nil
}

func (s *frontendPluginStore) DeletePluginFrontend(_ context.Context, pluginID string) error {
	delete(s.frontends, pluginID)
	return nil
}

func TestHandlePluginFrontendServesEntrypointAndAsset(t *testing.T) {
	t.Parallel()

	blobs := &memoryBlobStore{data: map[string][]byte{
		"front/index.html": []byte("<html>ok</html>"),
		"front/app.js":     []byte("console.log('ok')"),
	}}
	store := &frontendPluginStore{
		frontends: map[string]PluginFrontendRecord{
			"schedule": {
				PluginID:   "schedule",
				Entrypoint: "index.html",
				Assets: []PluginFrontendAsset{
					{Path: "index.html", Key: "front/index.html", ContentType: "text/html; charset=utf-8", Checksum: frontendAssetChecksum([]byte("<html>ok</html>")), Size: 15},
					{Path: "app.js", Key: "front/app.js", ContentType: "text/javascript; charset=utf-8", Checksum: frontendAssetChecksum([]byte("console.log('ok')")), Size: 17},
				},
			},
		},
	}
	handler := &AdminHandler{store: store, blobs: blobs}

	req := httptest.NewRequest(http.MethodGet, "/plugins/schedule/app/", nil)
	rec := httptest.NewRecorder()
	handler.handlePluginFrontend(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("entrypoint status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "<html>ok</html>" {
		t.Fatalf("entrypoint body = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/plugins/schedule/app/app.js", nil)
	rec = httptest.NewRecorder()
	handler.handlePluginFrontend(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("asset status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "console.log('ok')" {
		t.Fatalf("asset body = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/plugins/schedule/app/app.js", nil)
	req.Header.Set("If-None-Match", `"`+frontendAssetChecksum([]byte("console.log('ok')"))+`"`)
	rec = httptest.NewRecorder()
	handler.handlePluginFrontend(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("etag status = %d, want %d", rec.Code, http.StatusNotModified)
	}
}

func TestHandleListPluginsIncludesFrontendSummary(t *testing.T) {
	t.Parallel()

	store := &frontendPluginStore{
		testPluginStore: testPluginStore{
			records: map[string]PluginRecord{
				"schedule": {ID: "schedule", Enabled: false},
			},
			metadata: map[string]PluginMetadataRecord{
				"schedule": {PluginID: "schedule", Name: "Schedule", Version: "1.0.0"},
			},
		},
		frontends: map[string]PluginFrontendRecord{
			"schedule": {
				PluginID:   "schedule",
				Entrypoint: "index.html",
				Assets: []PluginFrontendAsset{
					{Path: "index.html", Key: "front/index.html", Size: 15},
					{Path: "app.js", Key: "front/app.js", Size: 17},
				},
			},
		},
	}
	handler := &AdminHandler{store: store, manager: plugin.NewManager()}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/plugins", nil)
	rec := httptest.NewRecorder()
	handler.handleListPlugins(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body []struct {
		ID       string                 `json:"id"`
		Frontend *pluginFrontendSummary `json:"frontend"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("plugins len = %d, want 1", len(body))
	}
	if body[0].ID != "schedule" {
		t.Fatalf("plugin id = %q, want schedule", body[0].ID)
	}
	if body[0].Frontend == nil {
		t.Fatal("frontend summary was not returned")
	}
	if body[0].Frontend.URL != "/plugins/schedule/app/" {
		t.Fatalf("frontend url = %q", body[0].Frontend.URL)
	}
	if body[0].Frontend.Assets != 2 {
		t.Fatalf("frontend assets = %d, want 2", body[0].Frontend.Assets)
	}
}

func TestInstallPersistsStagedPluginFrontend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	blobs := &memoryBlobStore{}
	wasmKey := "schedule_1.0.0.wasm"
	if err := blobs.Put(ctx, wasmKey, bytes.NewReader([]byte("wasm")), 4); err != nil {
		t.Fatalf("put wasm: %v", err)
	}
	staged, err := putPluginFrontendAssets(ctx, blobs, "schedule", []pluginFrontendFile{
		{Path: "index.html", Data: []byte("<html>ok</html>"), ContentType: "text/html; charset=utf-8", Size: 15},
	})
	if err != nil {
		t.Fatalf("putPluginFrontendAssets() error = %v", err)
	}
	if err := saveStagedPluginFrontendManifest(ctx, blobs, wasmKey, staged); err != nil {
		t.Fatalf("saveStagedPluginFrontendManifest() error = %v", err)
	}

	store := &frontendPluginStore{}
	svc := NewPluginLifecycleService(
		store,
		blobs,
		nil,
		plugin.NewManager(),
		nil,
		nil,
		nil,
		nil,
		nil,
		PluginLifecycleOptions{},
	)
	svc.loader = &testLifecycleLoader{
		loadPlugin: &testLifecyclePlugin{id: "schedule", name: "Schedule", version: "1.0.0"},
	}

	if _, err := svc.Install(ctx, "schedule", wasmKey, nil); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	frontend, ok := store.frontends["schedule"]
	if !ok {
		t.Fatal("frontend record was not saved")
	}
	if len(frontend.Assets) != 1 || frontend.Assets[0].Path != "index.html" {
		t.Fatalf("frontend assets = %#v", frontend.Assets)
	}
}

func TestPutPluginFrontendAssetsReusesUnchangedAsset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	blobs := &memoryBlobStore{data: map[string][]byte{
		"old/app.js": []byte("console.log('ok')"),
	}}
	previous := []PluginFrontendAsset{
		{
			Path:     "app.js",
			Key:      "old/app.js",
			Checksum: frontendAssetChecksum([]byte("console.log('ok')")),
			Size:     int64(len("console.log('ok')")),
		},
	}

	staged, err := putPluginFrontendAssetsReusing(ctx, blobs, "schedule", []pluginFrontendFile{
		{Path: "app.js", Data: []byte("console.log('ok')"), ContentType: "text/javascript; charset=utf-8", Size: int64(len("console.log('ok')"))},
		{Path: "index.html", Data: []byte("<html>ok</html>"), ContentType: "text/html; charset=utf-8", Size: int64(len("<html>ok</html>"))},
	}, previous)
	if err != nil {
		t.Fatalf("putPluginFrontendAssetsReusing() error = %v", err)
	}

	if len(staged.Assets) != 2 {
		t.Fatalf("assets len = %d, want 2", len(staged.Assets))
	}
	if staged.Assets[0].Key != "old/app.js" {
		t.Fatalf("reused key = %q, want old/app.js", staged.Assets[0].Key)
	}
	if _, ok := blobs.data["old/app.js"]; !ok {
		t.Fatal("old blob was removed")
	}
}
