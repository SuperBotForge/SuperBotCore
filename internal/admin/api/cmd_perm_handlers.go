package api

import (
	"net/http"

	"SuperBotGo/internal/weborigin"
)

type PolicyInvalidator interface {
	InvalidateCommandPolicy(pluginID, commandName string)
}

type CommandPermHandler struct {
	store       CommandPermStore
	invalidator PolicyInvalidator
}

func NewCommandPermHandler(store CommandPermStore, invalidator ...PolicyInvalidator) *CommandPermHandler {
	h := &CommandPermHandler{store: store}
	if len(invalidator) > 0 {
		h.invalidator = invalidator[0]
	}
	return h
}

func (h *CommandPermHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/plugins/{id}/commands/settings", h.handleListSettings)
	mux.HandleFunc("GET /api/admin/plugins/{id}/frontend-origins", h.handleGetPluginFrontendOrigins)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/frontend-origins", h.handleSetPluginFrontendOrigins)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/enabled", h.handleSetEnabled)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/access", h.handleSetAccess)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/policy", h.handleSetPolicy)
	mux.HandleFunc("PUT /api/admin/plugins/{id}/commands/{cmd}/origins", h.handleSetAllowedOrigins)
}

func (h *CommandPermHandler) invalidate(pluginID, cmd string) {
	if h.invalidator != nil {
		h.invalidator.InvalidateCommandPolicy(pluginID, cmd)
	}
}

func (h *CommandPermHandler) handleListSettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []CommandSetting{})
		return
	}
	pluginID := r.PathValue("id")
	settings, err := h.store.ListCommandSettings(r.Context(), pluginID)
	if err != nil {
		writeJSON(w, http.StatusOK, []CommandSetting{})
		return
	}
	if settings == nil {
		settings = []CommandSetting{}
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *CommandPermHandler) handleGetPluginFrontendOrigins(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("id")
	if h.store == nil {
		writeJSON(w, http.StatusOK, PluginFrontendOrigins{
			PluginID:       pluginID,
			AllowedOrigins: []string{},
		})
		return
	}

	setting, found, err := h.store.GetPluginFrontendOrigins(r.Context(), pluginID)
	if err != nil {
		writeJSON(w, http.StatusOK, PluginFrontendOrigins{
			PluginID:       pluginID,
			AllowedOrigins: []string{},
		})
		return
	}
	if !found {
		setting = PluginFrontendOrigins{
			PluginID:       pluginID,
			AllowedOrigins: []string{},
		}
	}
	writeJSON(w, http.StatusOK, setting)
}

func (h *CommandPermHandler) handleSetEnabled(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if err := h.store.SetCommandEnabled(r.Context(), pluginID, cmd, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update command setting")
		return
	}
	h.invalidate(pluginID, cmd)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *CommandPermHandler) handleSetAccess(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		AllowUserKeys    bool `json:"allow_user_keys"`
		AllowServiceKeys bool `json:"allow_service_keys"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if err := h.store.SetTriggerAccess(r.Context(), pluginID, cmd, body.AllowUserKeys, body.AllowServiceKeys); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update trigger access settings")
		return
	}
	h.invalidate(pluginID, cmd)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *CommandPermHandler) handleSetPolicy(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		Expression string `json:"expression"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	if err := h.store.SetPolicyExpression(r.Context(), pluginID, cmd, body.Expression); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save policy expression")
		return
	}
	h.invalidate(pluginID, cmd)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *CommandPermHandler) handleSetPluginFrontendOrigins(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")

	var body struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	origins, err := weborigin.CanonicalizeList(body.AllowedOrigins)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid allowed origin: "+err.Error())
		return
	}

	if err := h.store.SetPluginFrontendOrigins(r.Context(), pluginID, origins); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save frontend origins")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (h *CommandPermHandler) handleSetAllowedOrigins(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "requires PostgreSQL")
		return
	}
	pluginID := r.PathValue("id")
	cmd := r.PathValue("cmd")

	var body struct {
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if !decodeJSONBody(w, r, &body) {
		return
	}

	origins, err := weborigin.CanonicalizeList(body.AllowedOrigins)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid allowed origin: "+err.Error())
		return
	}

	if err := h.store.SetAllowedOrigins(r.Context(), pluginID, cmd, origins); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save allowed origins")
		return
	}
	h.invalidate(pluginID, cmd)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
