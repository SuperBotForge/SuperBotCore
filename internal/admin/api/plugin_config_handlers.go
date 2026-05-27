package api

import (
	"context"
	"encoding/json"
	"net/http"

	"SuperBotGo/internal/wasm/adapter"
)

func (h *AdminHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	result, err := h.lifecycle.UpdateConfig(r.Context(), pluginID, body.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": result.Status})
}

func (h *AdminHandler) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	var body struct {
		Config json.RawMessage `json:"config"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if err := h.lifecycle.ValidateConfig(r.Context(), pluginID, body.Config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"valid": "true"})
}

func (h *AdminHandler) handleListPlugins(w http.ResponseWriter, r *http.Request) {
	allPlugins := h.manager.All()
	records, _ := h.store.ListPlugins(r.Context())

	type pluginInfo struct {
		ID       string                 `json:"id"`
		Name     string                 `json:"name"`
		Version  string                 `json:"version"`
		Type     string                 `json:"type"`
		Status   string                 `json:"status"`
		Triggers int                    `json:"triggers"`
		Frontend *pluginFrontendSummary `json:"frontend,omitempty"`
	}

	result := make([]pluginInfo, 0, len(allPlugins)+len(records))

	for id, p := range allPlugins {
		pType := "go"
		triggerCount := len(p.Commands())
		if wp, ok := p.(*adapter.WasmPlugin); ok {
			pType = "wasm"
			triggerCount = len(wp.Meta().Triggers)
		}
		result = append(result, pluginInfo{
			ID:       id,
			Name:     p.Name(),
			Version:  p.Version(),
			Type:     pType,
			Status:   "active",
			Triggers: triggerCount,
			Frontend: h.pluginFrontendSummary(r.Context(), id),
		})
	}

	for _, rec := range records {
		if _, active := allPlugins[rec.ID]; !active {
			info := pluginInfo{
				ID:     rec.ID,
				Type:   "wasm",
				Status: "disabled",
			}
			if meta, err := h.store.GetPluginMetadata(r.Context(), rec.ID); err == nil {
				info.Name = meta.Name
				info.Version = meta.Version
			}
			info.Frontend = h.pluginFrontendSummary(r.Context(), rec.ID)
			result = append(result, info)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *AdminHandler) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")

	p, _ := h.manager.Get(pluginID)

	record, storeErr := h.store.GetPlugin(r.Context(), pluginID)

	if p == nil && storeErr != nil {
		writeError(w, http.StatusNotFound, "plugin not found")
		return
	}

	resp := map[string]interface{}{"id": pluginID}

	if p != nil {
		pType := "go"
		if wp, ok := p.(*adapter.WasmPlugin); ok {
			pType = "wasm"
			resp["meta"] = wp.Meta()
		}
		resp["name"] = p.Name()
		resp["version"] = p.Version()
		resp["type"] = pType
		resp["status"] = "active"
		cmds := make([]cmdInfo, 0, len(p.Commands()))
		for _, def := range p.Commands() {
			cmds = append(cmds, cmdInfo{
				Name:         def.Name,
				Descriptions: def.Descriptions,
				Description:  def.Description,
			})
		}
		resp["commands"] = cmds
	}

	if storeErr == nil {
		resp["config"] = record.ConfigJSON
		resp["wasm_hash"] = record.WasmHash
		resp["installed_at"] = record.InstalledAt
		resp["updated_at"] = record.UpdatedAt
		if frontend := h.pluginFrontendSummary(r.Context(), pluginID); frontend != nil {
			resp["frontend"] = frontend
		}
		if !record.Enabled {
			resp["status"] = "disabled"
			if meta, err := h.store.GetPluginMetadata(r.Context(), pluginID); err == nil {
				resp["name"] = meta.Name
				resp["version"] = meta.Version
				if len(meta.MetaJSON) > 0 {
					var parsedMeta map[string]any
					if json.Unmarshal(meta.MetaJSON, &parsedMeta) == nil {
						resp["meta"] = parsedMeta
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) pluginFrontendSummary(ctx context.Context, pluginID string) *pluginFrontendSummary {
	frontendStore, ok := pluginFrontendStoreFrom(h.store)
	if !ok {
		return nil
	}
	frontend, err := frontendStore.GetPluginFrontend(ctx, pluginID)
	if err != nil {
		return nil
	}
	summary := pluginFrontendSummaryFromRecord(frontend)
	return &summary
}
