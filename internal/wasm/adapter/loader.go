package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/trigger"
	"SuperBotGo/internal/wasm/hostapi"
	"SuperBotGo/internal/wasm/registry"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

var pluginDrainTimeout = wasmrt.PluginDrainTimeout

var _ hostapi.PluginRegistry = (*Loader)(nil)

type Loader struct {
	mu              sync.RWMutex
	rt              *wasmrt.Runtime
	hostAPI         *hostapi.HostAPI
	messageSend     MessageSendFunc
	plugins         map[string]*loadedPlugin
	triggerRegistry *trigger.Registry
	metrics         *metrics.Metrics
	registry        *registry.PluginRegistry
	strictMigrate   bool
}

type loadedPlugin struct {
	plugin         *WasmPlugin
	compiled       *wasmrt.CompiledModule
	config         json.RawMessage
	draining       atomic.Bool
	activeRequests atomic.Int64
	drained        chan struct{}
}

type preparedPlugin struct {
	meta      wasmrt.PluginMeta
	compiled  *wasmrt.CompiledModule
	config    json.RawMessage
	plugin    *WasmPlugin
	wasmBytes []byte
}

type reloadPlan struct {
	pluginID       string
	old            *loadedPlugin
	config         json.RawMessage
	oldVersion     string
	oldPermissions []string
}

func NewLoader(rt *wasmrt.Runtime, hostAPI *hostapi.HostAPI, messageSend MessageSendFunc) *Loader {
	return &Loader{
		rt:            rt,
		hostAPI:       hostAPI,
		messageSend:   messageSend,
		plugins:       make(map[string]*loadedPlugin),
		strictMigrate: true,
	}
}
func (l *Loader) SetMetrics(m *metrics.Metrics)            { l.metrics = m }
func (l *Loader) SetTriggerRegistry(tr *trigger.Registry)  { l.triggerRegistry = tr }
func (l *Loader) SetRegistry(reg *registry.PluginRegistry) { l.registry = reg }
func (l *Loader) Registry() *registry.PluginRegistry       { return l.registry }
func (l *Loader) SetStrictMigrate(strict bool)             { l.strictMigrate = strict }

func (l *Loader) LoadPlugin(ctx context.Context, wasmPath string, config json.RawMessage) (*WasmPlugin, error) {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm file %q: %w", wasmPath, err)
	}
	return l.LoadPluginFromBytes(ctx, data, config)
}

func (l *Loader) LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage) (*WasmPlugin, error) {
	prepared, err := l.preparePlugin(ctx, wasmBytes, config)
	if err != nil {
		return nil, err
	}

	l.registerLoadedPlugin(prepared, false)

	slog.Info("wasm: plugin loaded", "id", prepared.meta.ID, "name", prepared.meta.Name, "version", prepared.meta.Version)
	return prepared.plugin, nil
}

func (l *Loader) preparePlugin(ctx context.Context, wasmBytes []byte, config json.RawMessage) (*preparedPlugin, error) {
	compiled, err := l.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm plugin: %w", err)
	}

	meta, permissions, err := l.probePlugin(ctx, compiled)
	if err != nil {
		_ = compiled.Close(ctx)
		return nil, err
	}

	l.warnOnVersionChange(meta)

	if err := l.checkDependencies(ctx, compiled, &meta, wasmBytes); err != nil {
		return nil, err
	}

	if err := l.activatePlugin(ctx, compiled, meta, config, permissions); err != nil {
		return nil, err
	}

	compiled.EnablePool(l.rt.Config().PoolConfig())
	slog.Info("wasm: module pool enabled",
		"plugin", meta.ID,
		"max_concurrency", compiled.Pool().Stats().PoolSize)

	wp := &WasmPlugin{
		compiled:    compiled,
		meta:        meta,
		config:      cloneRawMessage(config),
		messageSend: l.messageSend,
	}

	return &preparedPlugin{
		meta:      meta,
		compiled:  compiled,
		config:    cloneRawMessage(config),
		plugin:    wp,
		wasmBytes: wasmBytes,
	}, nil
}

func (l *Loader) probePlugin(ctx context.Context, compiled *wasmrt.CompiledModule) (wasmrt.PluginMeta, []string, error) {
	const probeID = "_temp_probe"
	l.hostAPI.GrantPermissions(probeID, nil)
	compiled.ID = probeID

	meta, err := compiled.CallMeta(ctx)
	l.hostAPI.RevokePermissions(probeID)
	if err != nil {
		return wasmrt.PluginMeta{}, nil, fmt.Errorf("call meta: %w", err)
	}
	return meta, hostapi.PermissionsFromRequirements(meta.Requirements), nil
}

func (l *Loader) warnOnVersionChange(meta wasmrt.PluginMeta) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	existing, ok := l.plugins[meta.ID]
	if !ok {
		return
	}

	oldVer := existing.plugin.Version()
	newVer := meta.Version
	if oldVer == newVer {
		slog.Warn("wasm: plugin with the same version is already loaded",
			"id", meta.ID, "version", newVer)
		return
	}
	if registry.CompareVersions(newVer, oldVer) < 0 {
		slog.Warn("wasm: loading older plugin version than currently loaded",
			"id", meta.ID, "current_version", oldVer, "new_version", newVer)
	}
}

func (l *Loader) activatePlugin(ctx context.Context, compiled *wasmrt.CompiledModule, meta wasmrt.PluginMeta, config json.RawMessage, permissions []string) error {
	l.hostAPI.GrantPermissions(meta.ID, permissions)
	compiled.ID = meta.ID
	compiled.Version = meta.Version

	if len(config) > 0 {
		if err := ValidateConfigAgainstSchema(meta.ConfigSchema, config); err != nil {
			l.closeActivatedPlugin(ctx, meta.ID, compiled)
			return fmt.Errorf("plugin %q: %w", meta.ID, err)
		}
	}

	l.registerDatabases(meta.ID, config)
	if err := l.registerHTTPPolicies(meta.ID, meta.Requirements, config); err != nil {
		l.closeActivatedPlugin(ctx, meta.ID, compiled)
		return err
	}

	if err := l.validateDatabaseRequirements(meta); err != nil {
		l.closeActivatedPlugin(ctx, meta.ID, compiled)
		return err
	}

	if err := l.runPluginMigrations(ctx, meta); err != nil {
		l.closeActivatedPlugin(ctx, meta.ID, compiled)
		return err
	}

	if len(config) > 0 {
		if err := compiled.CallConfigure(ctx, config); err != nil {
			l.closeActivatedPlugin(ctx, meta.ID, compiled)
			return fmt.Errorf("configure plugin %q: %w", meta.ID, err)
		}
	}

	return nil
}

func (l *Loader) closeActivatedPlugin(ctx context.Context, pluginID string, compiled *wasmrt.CompiledModule) {
	_ = compiled.Close(ctx)
	if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
		sqlStore.UnregisterPlugin(pluginID)
	}
	l.hostAPI.RevokePermissions(pluginID)
}

func (l *Loader) validateDatabaseRequirements(meta wasmrt.PluginMeta) error {
	for _, req := range meta.Requirements {
		switch req.Type {
		case "database":
			dbName := req.Name
			if dbName == "" {
				dbName = "default"
			}
			if sqlStore := l.hostAPI.SQLStore(); sqlStore == nil || !sqlStore.HasDSN(meta.ID, dbName) {
				return fmt.Errorf("plugin %q requires database %q but its connection string is not configured", meta.ID, dbName)
			}
		case "plugin":
			if req.Target == "" {
				return fmt.Errorf("plugin %q declares plugin requirement without target", meta.ID)
			}
			if l.hostAPI.PluginRegistry() == nil {
				return fmt.Errorf("plugin %q requires inter-plugin RPC but it is disabled", meta.ID)
			}
		}
	}
	return nil
}

func (l *Loader) runPluginMigrations(ctx context.Context, meta wasmrt.PluginMeta) error {
	if len(meta.Migrations) == 0 {
		return nil
	}
	sqlStore := l.hostAPI.SQLStore()
	if sqlStore == nil {
		return nil
	}
	dsn := sqlStore.DSN(meta.ID, "default")
	if dsn == "" {
		return nil
	}
	if err := runPluginMigrations(ctx, meta.ID, dsn, meta.Migrations); err != nil {
		return fmt.Errorf("plugin %q migrations: %w", meta.ID, err)
	}
	return nil
}

func (l *Loader) registerLoadedPlugin(prepared *preparedPlugin, replace bool) {
	l.mu.Lock()
	_, existed := l.plugins[prepared.meta.ID]
	l.plugins[prepared.meta.ID] = &loadedPlugin{
		plugin:   prepared.plugin,
		compiled: prepared.compiled,
		config:   cloneRawMessage(prepared.config),
		drained:  make(chan struct{}),
	}
	l.mu.Unlock()

	if l.metrics != nil && !(replace && existed) {
		l.metrics.LoadedPluginsGauge.Inc()
	}

	if l.triggerRegistry != nil {
		if replace {
			l.triggerRegistry.UnregisterTriggers(prepared.meta.ID)
		}
		if len(prepared.meta.Triggers) > 0 {
			l.triggerRegistry.RegisterTriggers(prepared.meta.ID, prepared.meta.Triggers)
			slog.Info("wasm: registered triggers", "plugin", prepared.meta.ID, "count", len(prepared.meta.Triggers))
		}
	}

	l.registerInRegistry(&prepared.meta, prepared.wasmBytes)
}

// checkDependencies resolves plugin dependencies and verifies integrity if a registry is configured.
func (l *Loader) checkDependencies(ctx context.Context, compiled *wasmrt.CompiledModule, meta *wasmrt.PluginMeta, wasmBytes []byte) error {
	if l.registry != nil && len(meta.Dependencies) > 0 {
		installedPlugins := make([]registry.InstalledPlugin, 0, len(l.plugins))
		l.mu.RLock()
		for id, lp := range l.plugins {
			installedPlugins = append(installedPlugins, registry.InstalledPlugin{
				ID:      id,
				Version: lp.plugin.Version(),
			})
		}
		l.mu.RUnlock()

		tempEntry := registry.PluginEntry{
			ID:           meta.ID,
			Name:         meta.Name,
			Dependencies: convertDependencies(meta.Dependencies),
			Versions:     []registry.VersionEntry{{Version: meta.Version}},
		}
		l.registry.Register(tempEntry)

		if err := registry.ResolveDependencies(l.registry, meta.ID, meta.Version, installedPlugins); err != nil {
			_ = compiled.Close(ctx)
			return fmt.Errorf("plugin %q dependency check failed: %w", meta.ID, err)
		}
	}

	if l.registry != nil {
		if ve, err := l.registry.GetVersion(meta.ID, meta.Version); err == nil && ve.WasmHash != "" {
			if verifyErr := registry.VerifyOrError(wasmBytes, ve.WasmHash); verifyErr != nil {
				_ = compiled.Close(ctx)
				return fmt.Errorf("plugin %q: %w", meta.ID, verifyErr)
			}
			slog.Debug("wasm: integrity check passed", "plugin", meta.ID, "version", meta.Version)
		}
	}
	return nil
}

// registerDatabases reads the "databases" map from plugin config and registers
// each named DSN with the SQL store.
func (l *Loader) registerDatabases(pluginID string, config json.RawMessage) {
	sqlStore := l.hostAPI.SQLStore()
	if sqlStore == nil || len(config) == 0 {
		return
	}
	var cfgMap map[string]any
	if json.Unmarshal(config, &cfgMap) != nil {
		return
	}
	dbs, ok := cfgMap["databases"].(map[string]any)
	if !ok {
		return
	}
	for name, v := range dbs {
		if dsn, ok := v.(string); ok && dsn != "" {
			sqlStore.RegisterDSN(pluginID, name, dsn)
		}
	}
}

func (l *Loader) registerHTTPPolicies(pluginID string, requirements []wasmrt.RequirementDef, config json.RawMessage) error {
	policies, err := hostapi.ResolveHTTPPolicies(requirements, config)
	if err != nil {
		return fmt.Errorf("plugin %q http policy: %w", pluginID, err)
	}
	l.hostAPI.SetHTTPPolicies(pluginID, policies)
	return nil
}

func (l *Loader) registerInRegistry(meta *wasmrt.PluginMeta, wasmBytes []byte) {
	if l.registry == nil {
		return
	}
	hash := registry.SignModule(wasmBytes)
	l.registry.Register(registry.PluginEntry{
		ID:           meta.ID,
		Name:         meta.Name,
		Dependencies: convertDependencies(meta.Dependencies),
		Signature:    hash,
		Versions: []registry.VersionEntry{{
			Version:       meta.Version,
			WasmHash:      hash,
			UploadedAt:    time.Now(),
			MinSDKVersion: meta.SDKVersion,
		}},
	})
}

func convertDependencies(deps []wasmrt.DependencyDef) []registry.Dependency {
	result := make([]registry.Dependency, len(deps))
	for i, d := range deps {
		result[i] = registry.Dependency{
			PluginID:          d.PluginID,
			VersionConstraint: d.VersionConstraint,
		}
	}
	return result
}

func (l *Loader) ReloadPlugin(ctx context.Context, pluginID string, newWasmPath string, newConfig json.RawMessage) error {
	data, err := os.ReadFile(newWasmPath)
	if err != nil {
		return fmt.Errorf("read wasm file %q: %w", newWasmPath, err)
	}
	return l.ReloadPluginFromBytes(ctx, pluginID, data, newConfig)
}

func (l *Loader) ReloadPluginFromBytes(ctx context.Context, pluginID string, wasmBytes []byte, newConfig json.RawMessage) error {
	start := time.Now()
	reloadStatus := "ok"
	defer func() {
		dur := time.Since(start)
		if l.metrics != nil {
			l.metrics.PluginReloadTotal.WithLabelValues(pluginID, reloadStatus).Inc()
			l.metrics.PluginReloadDuration.WithLabelValues(pluginID).Observe(dur.Seconds())
		}
		slog.Info("wasm: plugin reload",
			"plugin_id", pluginID,
			"status", reloadStatus,
			"duration_ms", dur.Milliseconds(),
		)
	}()

	plan, err := l.beginReload(pluginID, newConfig)
	if err != nil {
		reloadStatus = "error"
		return err
	}

	prepared, err := l.preparePlugin(ctx, wasmBytes, plan.config)
	if err != nil {
		plan.old.draining.Store(false)
		reloadStatus = "error"
		return fmt.Errorf("reload plugin %q: load new version: %w", pluginID, err)
	}

	if prepared.meta.ID != pluginID {
		l.abortReload(ctx, plan, prepared)
		reloadStatus = "error"
		return fmt.Errorf("reload plugin %q: new module declares ID %q", pluginID, prepared.meta.ID)
	}

	if err := l.maybeRunReloadMigration(ctx, plan, prepared); err != nil {
		l.abortReload(ctx, plan, prepared)
		reloadStatus = "error"
		return err
	}

	l.registerLoadedPlugin(prepared, true)

	l.drainPlugin(plan.old, pluginID)

	if err := plan.old.compiled.Close(ctx); err != nil {
		slog.Error("wasm: error closing old compiled module during reload", "plugin", pluginID, "error", err)
	}

	slog.Info("wasm: plugin reloaded", "id", pluginID, "new_version", prepared.plugin.Version())
	return nil
}

func (l *Loader) beginReload(pluginID string, newConfig json.RawMessage) (*reloadPlan, error) {
	l.mu.RLock()
	old, ok := l.plugins[pluginID]
	l.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin %q not loaded", pluginID)
	}

	config := cloneRawMessage(newConfig)
	if config == nil {
		config = cloneRawMessage(old.config)
	}

	old.draining.Store(true)
	return &reloadPlan{
		pluginID:       pluginID,
		old:            old,
		config:         config,
		oldVersion:     old.plugin.Version(),
		oldPermissions: hostapi.PermissionsFromRequirements(old.plugin.Meta().Requirements),
	}, nil
}

func (l *Loader) abortReload(ctx context.Context, plan *reloadPlan, prepared *preparedPlugin) {
	if prepared != nil {
		l.closeActivatedPlugin(ctx, prepared.meta.ID, prepared.compiled)
	}
	l.restoreLoadedPluginState(plan.pluginID, plan.old.config, plan.oldPermissions)
	plan.old.draining.Store(false)
}

func (l *Loader) maybeRunReloadMigration(ctx context.Context, plan *reloadPlan, prepared *preparedPlugin) error {
	newVersion := prepared.plugin.Version()
	if plan.oldVersion == newVersion {
		return nil
	}

	slog.Info("wasm: plugin version changed, running migration",
		"plugin", plan.pluginID,
		"old_version", plan.oldVersion,
		"new_version", newVersion,
	)
	if err := prepared.compiled.CallMigrate(ctx, plan.oldVersion, newVersion); err != nil {
		if l.strictMigrate {
			return fmt.Errorf("reload plugin %q: migration failed: %w", plan.pluginID, err)
		}
		slog.Error("wasm: plugin migration failed (continuing because strict migrate is disabled)",
			"plugin", plan.pluginID,
			"old_version", plan.oldVersion,
			"new_version", newVersion,
			"error", err,
		)
		return nil
	}

	slog.Info("wasm: plugin migration completed successfully",
		"plugin", plan.pluginID,
		"old_version", plan.oldVersion,
		"new_version", newVersion,
	)
	return nil
}

func (l *Loader) restoreLoadedPluginState(pluginID string, config json.RawMessage, permissions []string) {
	l.hostAPI.GrantPermissions(pluginID, permissions)
	if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
		sqlStore.UnregisterPlugin(pluginID)
	}
	l.registerDatabases(pluginID, config)
	if wp, ok := l.GetPlugin(pluginID); ok {
		if err := l.registerHTTPPolicies(pluginID, wp.Meta().Requirements, config); err != nil {
			slog.Error("wasm: failed to restore http policies", "plugin", pluginID, "error", err)
		}
	}
}

func (l *Loader) ProbeMetadataFromBytes(ctx context.Context, wasmBytes []byte) (wasmrt.PluginMeta, error) {
	compiled, err := l.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("compile wasm plugin: %w", err)
	}
	defer compiled.Close(ctx)

	meta, _, err := l.probePlugin(ctx, compiled)
	if err != nil {
		return wasmrt.PluginMeta{}, err
	}
	return meta, nil
}

func (l *Loader) ReconfigurePlugin(ctx context.Context, pluginID string, config json.RawMessage) error {
	start := time.Now()
	status := "ok"
	defer func() {
		if l.metrics != nil {
			l.metrics.PluginReconfigureTotal.WithLabelValues(pluginID, status).Inc()
			l.metrics.PluginReconfigureDuration.WithLabelValues(pluginID).Observe(time.Since(start).Seconds())
		}
	}()

	l.mu.RLock()
	lp, ok := l.plugins[pluginID]
	l.mu.RUnlock()
	if !ok {
		status = "error"
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	if !lp.plugin.meta.SupportsReconfigure {
		status = "unsupported"
		return fmt.Errorf("plugin %q does not support reconfigure", pluginID)
	}
	if err := ValidateConfigAgainstSchema(lp.plugin.meta.ConfigSchema, config); err != nil {
		status = "invalid_config"
		return err
	}

	oldConfig := cloneRawMessage(lp.config)
	if sqlStore := l.hostAPI.SQLStore(); sqlStore != nil {
		sqlStore.UnregisterPlugin(pluginID)
	}
	l.registerDatabases(pluginID, config)
	if err := l.registerHTTPPolicies(pluginID, lp.plugin.meta.Requirements, config); err != nil {
		l.restoreLoadedPluginState(pluginID, oldConfig, hostapi.PermissionsFromRequirements(lp.plugin.meta.Requirements))
		status = "invalid_config"
		return err
	}
	if err := l.validateDatabaseRequirements(lp.plugin.meta); err != nil {
		l.restoreLoadedPluginState(pluginID, oldConfig, hostapi.PermissionsFromRequirements(lp.plugin.meta.Requirements))
		status = "missing_dependency"
		return err
	}

	if err := lp.compiled.CallReconfigure(ctx, oldConfig, config); err != nil {
		l.restoreLoadedPluginState(pluginID, oldConfig, hostapi.PermissionsFromRequirements(lp.plugin.meta.Requirements))
		status = "error"
		return fmt.Errorf("reconfigure plugin %q: %w", pluginID, err)
	}

	lp.config = cloneRawMessage(config)
	lp.plugin.SetConfig(config)
	return nil
}

// CheckVisibility calls check_visibility on the given plugin and returns
// visible command names. Returns nil, false if the plugin doesn't support it.
func (l *Loader) CheckVisibility(ctx context.Context, userID int64, pluginID string) ([]string, bool) {
	wp, release := l.AcquirePlugin(pluginID)
	if wp == nil {
		return nil, false
	}
	defer release()

	if !wp.SupportsVisibility() {
		return nil, false
	}

	names, err := wp.CheckVisibility(ctx, userID)
	if err != nil {
		slog.Warn("check_visibility failed", "plugin", pluginID, "error", err)
		return nil, false
	}
	return names, true
}

func (l *Loader) CallPlugin(ctx context.Context, target string, method string, params []byte) ([]byte, error) {
	wp, release := l.AcquirePlugin(target)
	if wp == nil {
		return nil, fmt.Errorf("plugin %q is not loaded", target)
	}
	defer release()

	if !wp.SupportsRPCMethod(method) {
		return nil, fmt.Errorf("plugin %q does not expose rpc method %q", target, method)
	}

	caller, _ := ctx.Value(wasmrt.PluginIDKey{}).(string)
	return wp.compiled.CallRPC(ctx, caller, method, params, wp.Config())
}
