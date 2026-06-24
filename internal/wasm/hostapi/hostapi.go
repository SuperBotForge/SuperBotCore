package hostapi

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/metrics"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type HostAPI struct {
	deps              Dependencies
	perms             *permissionStore
	httpPolicies      *httpPolicyStore
	httpPolicyEnabled bool
	maxFileStoreSize  int64
	metrics           *metrics.Metrics
	kvStore           *KVStore
	sqlStore          *SQLHandleStore
	rateLimits        map[string]int
}

func NewHostAPI(deps Dependencies) *HostAPI {
	return &HostAPI{
		deps:             deps,
		perms:            newPermissionStore(),
		httpPolicies:     newHTTPPolicyStore(),
		maxFileStoreSize: wasmrt.MaxFileStoreSize,
		kvStore:          NewKVStore(),
		sqlStore:         NewSQLHandleStore(),
	}
}

func (h *HostAPI) KVStore() *KVStore {
	return h.kvStore
}

func (h *HostAPI) SQLStore() *SQLHandleStore {
	return h.sqlStore
}

func (h *HostAPI) PluginRegistry() PluginRegistry {
	return h.deps.PluginRegistry
}

func (h *HostAPI) SetRateLimits(limits map[string]int) {
	h.rateLimits = limits
}

func (h *HostAPI) NewRateLimiterForPlugin(pluginID string) *RateLimiter {
	return NewRateLimiter(pluginID, h.rateLimits)
}

func (h *HostAPI) ContextWithRateLimiter(ctx context.Context, pluginID string) context.Context {
	rl := h.NewRateLimiterForPlugin(pluginID)
	return context.WithValue(ctx, rateLimiterKey{}, rl)
}

func (h *HostAPI) SetMetrics(m *metrics.Metrics) {
	h.metrics = m
}

func (h *HostAPI) SetFileStore(fs filestore.FileStore) {
	h.deps.FileStore = fs
}

func (h *HostAPI) SetEventBus(eb EventBus) {
	h.deps.Events = eb
}

func (h *HostAPI) SetNotifier(n Notifier) {
	h.deps.Notifier = n
}

func (h *HostAPI) SetPluginRegistry(reg PluginRegistry) {
	h.deps.PluginRegistry = reg
}

func (h *HostAPI) SetUserProvider(up UserProvider) {
	h.deps.UserProvider = up
}

func (h *HostAPI) SetMaxFileStoreSize(size int64) {
	if size <= 0 {
		h.maxFileStoreSize = wasmrt.MaxFileStoreSize
		return
	}
	h.maxFileStoreSize = size
}

func (h *HostAPI) RegisterHostModule(ctx context.Context, rt *wasmrt.Runtime) error {
	rt.AddContextHook(func(ctx context.Context, pluginID string) context.Context {
		ctx = h.ContextWithRateLimiter(ctx, pluginID)
		ctx = ContextWithTraceID(ctx, GenerateTraceID())
		if h.sqlStore != nil {
			traceID := TraceIDFromContext(ctx)
			context.AfterFunc(ctx, func() {
				h.sqlStore.CleanupExecution(pluginID, traceID)
			})
		}
		return ctx
	})

	builder := rt.Engine().NewHostModuleBuilder("env")

	builder = h.registerFunc(builder, "http_request", h.httpRequestFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "call_plugin", h.callPluginFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "publish_event", h.publishEventFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_get", h.kvGetFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_set", h.kvSetFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_delete", h.kvDeleteFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "kv_list", h.kvListFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "notify_user", h.notifyUserFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "notify_users", h.notifyUsersFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "notify_teacher", h.notifyTeacherFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "notify_chat", h.notifyChatFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	builder = h.registerFunc(builder, "notify_students", h.notifyStudentsFunc(),
		[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
		[]api.ValueType{api.ValueTypeI64})

	i32i32 := []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}
	i64 := []api.ValueType{api.ValueTypeI64}

	builder = h.registerFunc(builder, "sql_open", h.sqlOpenFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_close", h.sqlCloseFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_exec", h.sqlExecFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_query", h.sqlQueryFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_next", h.sqlNextFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_rows_close", h.sqlRowsCloseFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_begin", h.sqlBeginFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "sql_end", h.sqlEndFunc(), i32i32, i64)

	builder = h.registerFunc(builder, "file_meta", h.fileMetaFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "file_read", h.fileReadFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "file_read_into", h.fileReadIntoFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "file_url", h.fileURLFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "file_store", h.fileStoreFunc(), i32i32, i64)

	builder = h.registerFunc(builder, "user_info", h.userInfoFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "users_info", h.usersInfoFunc(), i32i32, i64)
	builder = h.registerFunc(builder, "list_users", h.listUsersFunc(), i32i32, i64)

	_, err := builder.Instantiate(ctx)
	return err
}

func (h *HostAPI) GrantPermissions(pluginID string, permissions []string) {
	h.perms.Grant(pluginID, permissions)
}

func (h *HostAPI) HasPermission(pluginID, permission string) bool {
	return h.perms.CheckPermission(pluginID, permission) == nil
}

func (h *HostAPI) RevokePermissions(pluginID string) {
	h.perms.Revoke(pluginID)
	h.httpPolicies.Delete(pluginID)
}

func pluginIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(wasmrt.PluginIDKey{}).(string); ok {
		return id
	}
	return "_unknown"
}

type hostCallStatus struct{ status string }

type hostCallStatusKey struct{}

func withHostCallStatus(ctx context.Context) (context.Context, *hostCallStatus) {
	s := &hostCallStatus{status: "ok"}
	return context.WithValue(ctx, hostCallStatusKey{}, s), s
}

func SetHostCallStatus(ctx context.Context, status string) {
	if s, ok := ctx.Value(hostCallStatusKey{}).(*hostCallStatus); ok {
		s.status = status
	}
}

func (h *HostAPI) registerFunc(builder wazero.HostModuleBuilder, name string, fn api.GoModuleFunc, params, results []api.ValueType) wazero.HostModuleBuilder {
	wrapped := func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		traceID := TraceIDFromContext(ctx)
		callChain := callChainFromContext(ctx)
		start := time.Now()

		ctx, callStatus := withHostCallStatus(ctx)

		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered in host function",
					"trace_id", traceID,
					"plugin_id", pluginID,
					"function", name,
					"panic", fmt.Sprintf("%v", r),
				)
				callStatus.status = "error"
				func() {
					defer func() {
						if r2 := recover(); r2 != nil {
							slog.Error("panic in recovery handler, returning zero result",
								"trace_id", traceID,
								"plugin_id", pluginID,
								"function", name,
								"panic", fmt.Sprintf("%v", r2),
							)
							stack[0] = 0
						}
					}()
					returnError(ctx, mod, stack,
						fmt.Errorf("host function %q panicked: %v", name, r))
				}()
			}

			dur := time.Since(start)
			status := callStatus.status

			if h.metrics != nil {
				h.metrics.HostAPITotal.WithLabelValues(pluginID, name, status).Inc()
				h.metrics.HostAPIDuration.WithLabelValues(pluginID, name).Observe(dur.Seconds())
			}

			logAttrs := []any{
				"trace_id", traceID,
				"plugin_id", pluginID,
				"function", name,
				"duration_ms", dur.Milliseconds(),
				"status", status,
			}
			if len(callChain) > 0 {
				logAttrs = append(logAttrs, "call_chain", strings.Join(callChain, " -> "))
			}
			switch name {
			case "sql_next", "sql_rows_close":
				slog.Debug("host api call", logAttrs...)
			default:
				slog.Info("host api call", logAttrs...)
			}
		}()

		if rl, ok := ctx.Value(rateLimiterKey{}).(*RateLimiter); ok {
			if err := rl.Allow(name); err != nil {
				callStatus.status = "rate_limited"
				slog.Warn("host function rate limit hit",
					"trace_id", traceID,
					"plugin_id", pluginID,
					"function", name,
				)
				returnError(ctx, mod, stack, err)
				return
			}
		}

		fn(ctx, mod, stack)
	}

	return builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(wrapped), params, results).
		WithName(name).
		Export(name)
}
