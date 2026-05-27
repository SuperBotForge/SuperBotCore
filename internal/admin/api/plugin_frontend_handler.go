package api

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
)

func (h *AdminHandler) handlePluginFrontend(w http.ResponseWriter, r *http.Request) {
	pluginID, assetPath, needsSlash, ok := parsePluginFrontendRequestPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if needsSlash {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	frontendStore, ok := pluginFrontendStoreFrom(h.store)
	if !ok {
		http.NotFound(w, r)
		return
	}
	record, err := frontendStore.GetPluginFrontend(r.Context(), pluginID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	asset, found := selectPluginFrontendAsset(record, assetPath)
	if !found {
		http.NotFound(w, r)
		return
	}

	contentType := asset.ContentType
	if contentType == "" {
		contentType = mime.TypeByExtension(path.Ext(asset.Path))
	}
	if asset.Checksum != "" {
		etag := `"` + asset.Checksum + `"`
		w.Header().Set("ETag", etag)
		if requestHasETag(r, etag) {
			w.Header().Set("Cache-Control", pluginFrontendCacheControl(asset.Path))
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	rc, err := h.blobs.Get(r.Context(), asset.Key)
	if err != nil {
		http.Error(w, "frontend asset not found", http.StatusNotFound)
		return
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, maxPluginFrontendAssetSize+1))
	if err != nil {
		http.Error(w, "failed to read frontend asset", http.StatusInternalServerError)
		return
	}
	if len(data) > maxPluginFrontendAssetSize {
		http.Error(w, "frontend asset too large", http.StatusInternalServerError)
		return
	}

	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", pluginFrontendCacheControl(asset.Path))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

func parsePluginFrontendRequestPath(requestPath string) (pluginID, assetPath string, needsSlash, ok bool) {
	if !strings.HasPrefix(requestPath, "/plugins/") {
		return "", "", false, false
	}
	rest := strings.TrimPrefix(requestPath, "/plugins/")
	id, route, found := strings.Cut(rest, "/")
	if !found || id == "" {
		return "", "", false, false
	}
	if route == "app" {
		return id, "", true, true
	}
	if !strings.HasPrefix(route, "app/") {
		return "", "", false, false
	}
	rawAssetPath := strings.TrimPrefix(route, "app/")
	cleanAssetPath, valid := cleanFrontendAssetPath(rawAssetPath)
	if !valid {
		return "", "", false, false
	}
	return id, cleanAssetPath, false, true
}

func selectPluginFrontendAsset(record PluginFrontendRecord, assetPath string) (PluginFrontendAsset, bool) {
	entrypoint := record.Entrypoint
	if entrypoint == "" {
		entrypoint = pluginFrontendEntrypoint
	}
	wantPath := assetPath
	if wantPath == "" {
		wantPath = entrypoint
	}

	for _, asset := range record.Assets {
		if asset.Path == wantPath {
			return asset, true
		}
	}

	if path.Ext(wantPath) != "" {
		return PluginFrontendAsset{}, false
	}
	for _, asset := range record.Assets {
		if asset.Path == entrypoint {
			return asset, true
		}
	}
	return PluginFrontendAsset{}, false
}

func pluginFrontendCacheControl(_ string) string {
	return "no-cache"
}

func requestHasETag(r *http.Request, etag string) bool {
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if strings.TrimSpace(candidate) == etag {
			return true
		}
	}
	return false
}
