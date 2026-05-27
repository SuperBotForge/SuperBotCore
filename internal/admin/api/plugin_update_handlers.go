package api

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

func (h *AdminHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	upload, ok := readPluginUploadFromForm(w, r)
	if !ok {
		return
	}
	wasmBytes := upload.WasmBytes

	meta, err := h.probeUploadedPlugin(r.Context(), wasmBytes)
	if err != nil {
		slog.Error("admin: invalid wasm module", "error", err)
		writeError(w, http.StatusBadRequest, "invalid wasm module")
		return
	}

	var existingVersion string
	if wp, ok := h.loader.GetPlugin(meta.ID); ok {
		existingVersion = wp.Version()
	} else if h.versions != nil {
		if vv, err := h.versions.ListVersions(r.Context(), meta.ID); err == nil && len(vv) > 0 {
			existingVersion = vv[0].Version
		}
	}

	wasmKey := fmt.Sprintf("%s_%s.wasm", meta.ID, meta.Version)
	if err := h.blobs.Put(r.Context(), wasmKey, bytes.NewReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		slog.Error("admin: failed to save wasm file", "key", wasmKey, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save wasm file")
		return
	}

	var frontendSummary *pluginFrontendSummary
	if len(upload.FrontendFiles) > 0 {
		staged, err := putPluginFrontendAssets(r.Context(), h.blobs, meta.ID, upload.FrontendFiles)
		if err != nil {
			_ = h.blobs.Delete(r.Context(), wasmKey)
			slog.Error("admin: failed to save plugin frontend assets", "plugin", meta.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save plugin frontend")
			return
		}
		if err := saveStagedPluginFrontendManifest(r.Context(), h.blobs, wasmKey, staged); err != nil {
			_ = h.blobs.Delete(r.Context(), wasmKey)
			deleteFrontendAssetsBestEffort(r.Context(), h.blobs, staged.Assets)
			slog.Error("admin: failed to save plugin frontend manifest", "plugin", meta.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to save plugin frontend")
			return
		}
		summary := pluginFrontendSummary{
			URL:        pluginFrontendAppURL(meta.ID),
			Entrypoint: staged.Entrypoint,
			Assets:     len(staged.Assets),
		}
		frontendSummary = &summary
	} else if err := h.blobs.Delete(r.Context(), pluginFrontendManifestKey(wasmKey)); err != nil {
		slog.Warn("admin: failed to delete stale plugin frontend manifest", "key", pluginFrontendManifestKey(wasmKey), "error", err)
	}

	writeJSON(w, http.StatusOK, uploadResponse{
		ID:              meta.ID,
		Name:            meta.Name,
		Version:         meta.Version,
		RPCMethods:      meta.RPCMethods,
		Triggers:        meta.Triggers,
		Requirements:    meta.Requirements,
		ConfigSchema:    meta.ConfigSchema,
		WasmKey:         wasmKey,
		WasmHash:        hashWASM(wasmBytes),
		ExistingVersion: existingVersion,
		Frontend:        frontendSummary,
	})
}

func (h *AdminHandler) handleUpdatePreview(w http.ResponseWriter, r *http.Request) {
	upload, ok := readPluginUploadFromForm(w, r)
	if !ok {
		return
	}

	preview, err := h.buildUpdatePreview(r.Context(), r.PathValue("id"), upload.WasmBytes)
	if err != nil {
		slog.Error("admin: failed to build plugin update preview", "id", r.PathValue("id"), "error", err)
		if _, storeErr := h.store.GetPlugin(r.Context(), r.PathValue("id")); storeErr != nil {
			writeError(w, http.StatusNotFound, "plugin not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

func (h *AdminHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	upload, ok := readPluginUploadFromForm(w, r)
	if !ok {
		return
	}
	changelog := strings.TrimSpace(r.FormValue("changelog"))
	result, err := h.lifecycle.UpdateWithFrontend(r.Context(), r.PathValue("id"), upload.WasmBytes, upload.FrontendFiles, changelog)
	if err != nil {
		slog.Error("admin: failed to update plugin", "id", r.PathValue("id"), "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}

func (h *AdminHandler) probeUploadedPlugin(ctx context.Context, wasmBytes []byte) (wasmrt.PluginMeta, error) {
	if h.loader == nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("wasm loader is not configured")
	}
	meta, err := h.loader.ProbeMetadataFromBytes(ctx, wasmBytes)
	if err != nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("probe uploaded plugin metadata: %w", err)
	}
	return meta, nil
}
