package trigger

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin/contract"
	wasmrt "SuperBotGo/internal/wasm/runtime"
	"SuperBotGo/internal/weborigin"
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

type HTTPTriggerSetting struct {
	Enabled          bool
	AllowUserKeys    bool
	AllowServiceKeys bool
	PolicyExpression string
	AllowedOrigins   []string
}

type ServiceKeyPrincipal struct {
	ID int64
}

type resolvedHTTPPrincipal struct {
	authData *contract.HTTPAuthData
}

type pluginHTTPError struct {
	message string
}

func (e pluginHTTPError) Error() string {
	return e.message
}

type HTTPTriggerHandler struct {
	router   *Router
	registry *Registry
	basePath string
	metrics  *metrics.Metrics

	loadSetting           func(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, bool, error)
	authenticateUser      func(r *http.Request) (model.GlobalUserID, bool)
	authenticateUserToken func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)
	authenticateService   func(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error)
	evalPolicy            func(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error)
}

func NewHTTPTriggerHandler(router *Router, registry *Registry) *HTTPTriggerHandler {
	return &HTTPTriggerHandler{
		router:   router,
		registry: registry,
		basePath: "/api/triggers/http/",
	}
}

func (h *HTTPTriggerHandler) SetMetrics(m *metrics.Metrics) {
	h.metrics = m
}

func (h *HTTPTriggerHandler) SetSettingLoader(loader func(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, bool, error)) {
	h.loadSetting = loader
}

func (h *HTTPTriggerHandler) SetUserAuthenticator(fn func(r *http.Request) (model.GlobalUserID, bool)) {
	h.authenticateUser = fn
}

func (h *HTTPTriggerHandler) SetUserTokenAuthenticator(fn func(ctx context.Context, rawToken string) (model.GlobalUserID, bool, error)) {
	h.authenticateUserToken = fn
}

func (h *HTTPTriggerHandler) SetServiceAuthenticator(fn func(ctx context.Context, rawToken, pluginID, triggerName string) (ServiceKeyPrincipal, bool, error)) {
	h.authenticateService = fn
}

func (h *HTTPTriggerHandler) SetPolicyEvaluator(fn func(ctx context.Context, expression string, userID model.GlobalUserID) (bool, error)) {
	h.evalPolicy = fn
}

func (h *HTTPTriggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
	var pluginID string
	defer func() {
		duration := time.Since(start)
		if h.metrics != nil && pluginID != "" {
			h.metrics.HTTPTriggerDuration.WithLabelValues(pluginID, r.Method).Observe(duration.Seconds())
			h.metrics.HTTPTriggerTotal.WithLabelValues(pluginID, r.Method, strconv.Itoa(rec.statusCode)).Inc()
		}
		slog.Info("http trigger",
			"plugin_id", pluginID,
			"method", r.Method,
			"path", r.URL.Path,
			"status_code", rec.statusCode,
			"duration_ms", duration.Milliseconds(),
		)
	}()

	var err error
	pluginID, triggerPath, err := h.resolveRoute(r.URL.Path)
	if err != nil {
		http.Error(rec, err.Error(), http.StatusBadRequest)
		return
	}

	triggerMethod := routeLookupMethod(r)
	triggerName, err := h.registry.LookupHTTP(pluginID, triggerPath, triggerMethod)
	if err != nil {
		http.Error(rec, err.Error(), http.StatusNotFound)
		return
	}

	setting, err := h.resolveSetting(r.Context(), pluginID, triggerName)
	if err != nil {
		slog.Error("HTTP trigger: failed to load access setting", "plugin", pluginID, "trigger", triggerName, "error", err)
		http.Error(rec, "internal error", http.StatusInternalServerError)
		return
	}
	if !setting.Enabled {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	}

	if preflight, err := applyHTTPTriggerCORS(rec, r, setting, triggerMethod); err != nil {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	} else if preflight {
		rec.WriteHeader(http.StatusNoContent)
		return
	}

	principal, statusCode, err := h.resolvePrincipal(r, pluginID, triggerName, setting)
	if err != nil {
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		if redirectURL, ok := loginRedirectURL(r, statusCode, setting); ok {
			http.Redirect(rec, r, redirectURL, http.StatusFound)
			return
		}
		http.Error(rec, err.Error(), statusCode)
		return
	}

	event, err := h.buildHTTPEvent(r, pluginID, triggerName, triggerPath, principal)
	if err != nil {
		http.Error(rec, err.Error(), http.StatusBadRequest)
		return
	}

	httpResp, err := h.dispatchHTTPEvent(r.Context(), event)
	if err != nil {
		var pluginErr pluginHTTPError
		if errors.As(err, &pluginErr) {
			slog.Error("HTTP trigger plugin error", "plugin", pluginID, "trigger", triggerName, "error", pluginErr.Error())
			http.Error(rec, pluginErr.Error(), http.StatusInternalServerError)
			return
		}
		slog.Error("HTTP trigger dispatch failed", "plugin", pluginID, "trigger", triggerName, "error", err)
		http.Error(rec, "internal error", http.StatusInternalServerError)
		return
	}

	writeHTTPResponse(rec, httpResp)
}

func routeLookupMethod(r *http.Request) string {
	if isHTTPTriggerCORSPreflight(r) {
		return strings.TrimSpace(r.Header.Get("Access-Control-Request-Method"))
	}
	return r.Method
}

func isHTTPTriggerCORSPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions &&
		strings.TrimSpace(r.Header.Get("Origin")) != "" &&
		strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")) != ""
}

func applyHTTPTriggerCORS(w http.ResponseWriter, r *http.Request, setting HTTPTriggerSetting, triggerMethod string) (bool, error) {
	preflight := isHTTPTriggerCORSPreflight(r)
	originHeader := strings.TrimSpace(r.Header.Get("Origin"))
	if originHeader == "" {
		return false, nil
	}

	origin, err := weborigin.Canonicalize(originHeader)
	if err != nil {
		return preflight, err
	}
	if requestOriginMatches(r, origin) {
		return preflight, nil
	}
	if !weborigin.Contains(setting.AllowedOrigins, origin) {
		return preflight, fmt.Errorf("origin is not allowed")
	}

	header := w.Header()
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Credentials", "true")
	addVary(header, "Origin")
	if preflight {
		header.Set("Access-Control-Allow-Methods", strings.ToUpper(triggerMethod))
		if requestedHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")); requestedHeaders != "" {
			header.Set("Access-Control-Allow-Headers", requestedHeaders)
			addVary(header, "Access-Control-Request-Headers")
		}
		addVary(header, "Access-Control-Request-Method")
	}
	return preflight, nil
}

func requestOriginMatches(r *http.Request, origin string) bool {
	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	if host == "" {
		return false
	}
	requestOrigin, err := weborigin.Canonicalize(requestScheme(r) + "://" + host)
	if err != nil {
		return false
	}
	return requestOrigin == origin
}

func requestScheme(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		proto := strings.ToLower(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
		if proto == "http" || proto == "https" {
			return proto
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func addVary(header http.Header, value string) {
	for _, item := range header.Values("Vary") {
		for _, part := range strings.Split(item, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}

func (h *HTTPTriggerHandler) resolveRoute(path string) (pluginID, triggerPath string, err error) {
	remainder := strings.TrimPrefix(path, h.basePath)
	remainder = strings.TrimPrefix(remainder, "/")
	if remainder == "" {
		return "", "", fmt.Errorf("missing plugin ID in URL")
	}

	parts := strings.SplitN(remainder, "/", 2)
	pluginID = parts[0]
	if len(parts) > 1 {
		triggerPath = parts[1]
	}
	return pluginID, triggerPath, nil
}

func (h *HTTPTriggerHandler) buildHTTPEvent(r *http.Request, pluginID, triggerName, triggerPath string, principal resolvedHTTPPrincipal) (contract.Event, error) {
	triggerData, err := buildHTTPRequestData(r, triggerPath, principal)
	if err != nil {
		return contract.Event{}, err
	}

	dataJSON, _ := json.Marshal(triggerData)
	return contract.Event{
		ID:          generateID(),
		TriggerType: contract.TriggerHTTP,
		TriggerName: triggerName,
		PluginID:    pluginID,
		Timestamp:   time.Now().UnixMilli(),
		Data:        dataJSON,
	}, nil
}

func buildHTTPRequestData(r *http.Request, triggerPath string, principal resolvedHTTPPrincipal) (contract.HTTPTriggerData, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		return contract.HTTPTriggerData{}, fmt.Errorf("failed to read request body")
	}

	query := make(map[string]string, len(r.URL.Query()))
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return contract.HTTPTriggerData{
		Method:     r.Method,
		Path:       "/" + triggerPath,
		Query:      query,
		Headers:    headers,
		Body:       string(body),
		RemoteAddr: r.RemoteAddr,
		Auth:       principal.authData,
	}, nil
}

func (h *HTTPTriggerHandler) dispatchHTTPEvent(ctx context.Context, event contract.Event) (contract.HTTPResponseData, error) {
	if httpData, err := event.HTTP(); err == nil && httpData != nil && httpData.Auth != nil {
		ctx = context.WithValue(ctx, wasmrt.HTTPAuthDataKey{}, *httpData.Auth)
	}
	resp, err := h.router.RouteEvent(ctx, event)
	if err != nil {
		return contract.HTTPResponseData{}, err
	}
	if resp == nil {
		return contract.HTTPResponseData{}, nil
	}
	if resp.Error != "" {
		return contract.HTTPResponseData{}, pluginHTTPError{message: resp.Error}
	}
	if len(resp.Data) == 0 {
		return contract.HTTPResponseData{}, nil
	}

	var httpResp contract.HTTPResponseData
	if err := json.Unmarshal(resp.Data, &httpResp); err != nil {
		return contract.HTTPResponseData{}, fmt.Errorf("parse plugin HTTP response: %w", err)
	}
	return httpResp, nil
}

func writeHTTPResponse(w http.ResponseWriter, httpResp contract.HTTPResponseData) {
	for k, v := range httpResp.Headers {
		if isManagedCORSHeader(k) && w.Header().Get(k) != "" {
			continue
		}
		w.Header().Set(k, v)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	statusCode := httpResp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(httpResp.Body))
}

func isManagedCORSHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Access-Control-Allow-Origin",
		"Access-Control-Allow-Credentials",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers":
		return true
	default:
		return false
	}
}

func (h *HTTPTriggerHandler) resolveSetting(ctx context.Context, pluginID, triggerName string) (HTTPTriggerSetting, error) {
	setting := HTTPTriggerSetting{
		Enabled:          true,
		AllowUserKeys:    true,
		AllowServiceKeys: false,
	}
	if h.loadSetting == nil {
		return setting, nil
	}
	loaded, found, err := h.loadSetting(ctx, pluginID, triggerName)
	if err != nil {
		return HTTPTriggerSetting{}, err
	}
	if found {
		return loaded, nil
	}
	return setting, nil
}

func (h *HTTPTriggerHandler) resolvePrincipal(r *http.Request, pluginID, triggerName string, setting HTTPTriggerSetting) (resolvedHTTPPrincipal, int, error) {
	allowAnonymous := allowsAnonymousHTTPAccess(setting)

	if token, ok := bearerToken(r); ok {
		principal, statusCode, err := h.resolveBearerPrincipal(r.Context(), token, pluginID, triggerName, setting)
		if err == nil {
			return principal, 0, nil
		}
		if allowAnonymous && statusCode == http.StatusForbidden {
			return resolvedHTTPPrincipal{}, 0, nil
		}
		return resolvedHTTPPrincipal{}, statusCode, err
	}

	if h.authenticateUser != nil {
		if userID, ok := h.authenticateUser(r); ok {
			principal, statusCode, err := h.authorizeUserPrincipal(r.Context(), pluginID, triggerName, setting, userID)
			if err == nil {
				return principal, 0, nil
			}
			if allowAnonymous && statusCode == http.StatusForbidden {
				return resolvedHTTPPrincipal{}, 0, nil
			}
			return resolvedHTTPPrincipal{}, statusCode, err
		}
	}

	if allowAnonymous {
		return resolvedHTTPPrincipal{}, 0, nil
	}

	return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
}

func (h *HTTPTriggerHandler) resolveBearerPrincipal(ctx context.Context, rawToken, pluginID, triggerName string, setting HTTPTriggerSetting) (resolvedHTTPPrincipal, int, error) {
	if h.authenticateUserToken != nil {
		userID, ok, err := h.authenticateUserToken(ctx, rawToken)
		if err != nil {
			return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("internal error")
		}
		if ok {
			return h.authorizeUserPrincipal(ctx, pluginID, triggerName, setting, userID)
		}
	}

	if h.authenticateService == nil {
		return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	principal, ok, err := h.authenticateService(ctx, rawToken, pluginID, triggerName)
	if err != nil {
		return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("internal error")
	}
	if !ok {
		return resolvedHTTPPrincipal{}, http.StatusUnauthorized, fmt.Errorf("authentication required")
	}
	if !setting.AllowServiceKeys {
		return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
	}
	return resolvedHTTPPrincipal{
		authData: &contract.HTTPAuthData{
			Kind:         contract.HTTPAuthService,
			ServiceKeyID: principal.ID,
		},
	}, 0, nil
}

func (h *HTTPTriggerHandler) authorizeUserPrincipal(ctx context.Context, pluginID, triggerName string, setting HTTPTriggerSetting, userID model.GlobalUserID) (resolvedHTTPPrincipal, int, error) {
	if !setting.AllowUserKeys {
		return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
	}
	if setting.PolicyExpression != "" {
		if h.evalPolicy == nil {
			return resolvedHTTPPrincipal{}, http.StatusInternalServerError, fmt.Errorf("authorization unavailable")
		}
		allowed, err := h.evalPolicy(ctx, setting.PolicyExpression, userID)
		if err != nil {
			slog.Warn("HTTP trigger policy expression error",
				"plugin", pluginID,
				"trigger", triggerName,
				"error", err)
			return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
		}
		if !allowed {
			return resolvedHTTPPrincipal{}, http.StatusForbidden, fmt.Errorf("forbidden")
		}
	}
	return resolvedHTTPPrincipal{
		authData: &contract.HTTPAuthData{
			Kind:   contract.HTTPAuthUser,
			UserID: userID,
		},
	}, 0, nil
}

func bearerToken(r *http.Request) (string, bool) {
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
	if token == "" {
		return "", false
	}
	return token, true
}

func loginRedirectURL(r *http.Request, statusCode int, setting HTTPTriggerSetting) (string, bool) {
	if statusCode != http.StatusUnauthorized {
		return "", false
	}
	if !setting.AllowUserKeys {
		return "", false
	}
	if _, ok := bearerToken(r); ok {
		return "", false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}
	if !acceptsHTML(r) {
		return "", false
	}

	return "/api/auth/tsu/start?return_to=" + url.QueryEscape(requestReturnTo(r)), true
}

func acceptsHTML(r *http.Request) bool {
	for _, value := range r.Header.Values("Accept") {
		for _, part := range strings.Split(value, ",") {
			if mediaType := strings.TrimSpace(strings.SplitN(part, ";", 2)[0]); mediaType == "text/html" {
				return true
			}
		}
	}
	return false
}

func requestReturnTo(r *http.Request) string {
	if r.URL == nil {
		return "/"
	}
	if raw := r.URL.RequestURI(); raw != "" {
		return raw
	}
	if r.URL.Path == "" {
		return "/"
	}
	return r.URL.Path
}

func allowsAnonymousHTTPAccess(setting HTTPTriggerSetting) bool {
	return !setting.AllowUserKeys && !setting.AllowServiceKeys && strings.TrimSpace(setting.PolicyExpression) == ""
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version 4 (random) and variant bits per RFC 4122.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
