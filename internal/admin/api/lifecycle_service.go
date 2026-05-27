package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/pubsub"
	"SuperBotGo/internal/wasm/adapter"
	"SuperBotGo/internal/wasm/hostapi"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type PluginLifecycleOptions struct {
	ReconfigureEnabled bool
}

type LifecycleResult struct {
	PluginID   string `json:"plugin_id"`
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	OldVersion string `json:"old_version,omitempty"`
	NewVersion string `json:"new_version,omitempty"`
	Status     string `json:"status"`
}

type PluginLifecycleService struct {
	store       PluginStore
	blobs       BlobStore
	loader      lifecycleLoader
	manager     *plugin.Manager
	hostAPI     *hostapi.HostAPI
	stateMgr    StateManagerRegistrar
	cmdStore    CommandPermStore
	versions    VersionStore
	bus         *pubsub.Bus
	invalidator PolicyInvalidator
	opts        PluginLifecycleOptions
}

type lifecyclePlugin interface {
	plugin.Plugin
	Meta() wasmrt.PluginMeta
}

type lifecycleLoader interface {
	GetPlugin(pluginID string) (lifecyclePlugin, bool)
	LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage) (lifecyclePlugin, error)
	ReloadPluginFromBytes(ctx context.Context, pluginID string, wasmBytes []byte, newConfig json.RawMessage) error
	ProbeMetadataFromBytes(ctx context.Context, wasmBytes []byte) (wasmrt.PluginMeta, error)
	UnloadPlugin(ctx context.Context, pluginID string) error
	ReconfigurePlugin(ctx context.Context, pluginID string, config json.RawMessage) error
	DropPluginData(ctx context.Context, pluginID string) error
}

type adapterLifecycleLoader struct {
	loader *adapter.Loader
}

func (l adapterLifecycleLoader) GetPlugin(pluginID string) (lifecyclePlugin, bool) {
	return l.loader.GetPlugin(pluginID)
}

func (l adapterLifecycleLoader) LoadPluginFromBytes(ctx context.Context, wasmBytes []byte, config json.RawMessage) (lifecyclePlugin, error) {
	return l.loader.LoadPluginFromBytes(ctx, wasmBytes, config)
}

func (l adapterLifecycleLoader) ReloadPluginFromBytes(ctx context.Context, pluginID string, wasmBytes []byte, newConfig json.RawMessage) error {
	return l.loader.ReloadPluginFromBytes(ctx, pluginID, wasmBytes, newConfig)
}

func (l adapterLifecycleLoader) ProbeMetadataFromBytes(ctx context.Context, wasmBytes []byte) (wasmrt.PluginMeta, error) {
	return l.loader.ProbeMetadataFromBytes(ctx, wasmBytes)
}

func (l adapterLifecycleLoader) UnloadPlugin(ctx context.Context, pluginID string) error {
	return l.loader.UnloadPlugin(ctx, pluginID)
}

func (l adapterLifecycleLoader) ReconfigurePlugin(ctx context.Context, pluginID string, config json.RawMessage) error {
	return l.loader.ReconfigurePlugin(ctx, pluginID, config)
}

func (l adapterLifecycleLoader) DropPluginData(ctx context.Context, pluginID string) error {
	return l.loader.DropPluginData(ctx, pluginID)
}

func NewPluginLifecycleService(
	store PluginStore,
	blobs BlobStore,
	loader *adapter.Loader,
	manager *plugin.Manager,
	hostAPI *hostapi.HostAPI,
	stateMgr StateManagerRegistrar,
	cmdStore CommandPermStore,
	versions VersionStore,
	bus *pubsub.Bus,
	opts PluginLifecycleOptions,
	invalidator ...PolicyInvalidator,
) *PluginLifecycleService {
	svc := &PluginLifecycleService{
		store:    store,
		blobs:    blobs,
		loader:   adapterLifecycleLoader{loader: loader},
		manager:  manager,
		hostAPI:  hostAPI,
		stateMgr: stateMgr,
		cmdStore: cmdStore,
		versions: versions,
		bus:      bus,
		opts:     opts,
	}
	if len(invalidator) > 0 {
		svc.invalidator = invalidator[0]
	}
	return svc
}

func (s *PluginLifecycleService) HandleEvent(event pubsub.AdminEvent) {
	ctx := context.Background()
	slog.Info("pubsub: received lifecycle event", "type", event.Type, "plugin", event.PluginID, "from", event.InstanceID)

	switch event.Type {
	case pubsub.EventPluginInstalled, pubsub.EventPluginEnabled:
		if err := s.loadFromStore(ctx, event.PluginID); err != nil {
			slog.Error("pubsub: failed to load plugin", "plugin", event.PluginID, "error", err)
		}
	case pubsub.EventPluginDisabled, pubsub.EventPluginUninstalled:
		s.removeLocal(ctx, event.PluginID)
	case pubsub.EventPluginUpdated:
		if err := s.reloadFromStore(ctx, event.PluginID); err != nil {
			slog.Error("pubsub: failed to reload plugin", "plugin", event.PluginID, "error", err)
		}
	case pubsub.EventConfigChanged:
		if err := s.applyStoredConfig(ctx, event.PluginID); err != nil {
			slog.Error("pubsub: failed to apply plugin config", "plugin", event.PluginID, "error", err)
		}
	default:
		slog.Warn("pubsub: unknown lifecycle event", "type", event.Type)
	}
}

func (s *PluginLifecycleService) ValidateConfig(ctx context.Context, pluginID string, config json.RawMessage) error {
	if wp, ok := s.loader.GetPlugin(pluginID); ok {
		return adapter.ValidateConfigAgainstSchema(wp.Meta().ConfigSchema, config)
	}

	meta, err := s.store.GetPluginMetadata(ctx, pluginID)
	if err != nil {
		return fmt.Errorf("plugin %q metadata not found: %w", pluginID, err)
	}
	return adapter.ValidateConfigAgainstSchema(meta.ConfigSchema, config)
}

func (s *PluginLifecycleService) Install(ctx context.Context, pluginID string, wasmKey string, config json.RawMessage) (LifecycleResult, error) {
	if _, err := s.store.GetPlugin(ctx, pluginID); err == nil {
		return LifecycleResult{}, fmt.Errorf("plugin %q already installed", pluginID)
	}

	wasmBytes, err := s.readBlob(ctx, wasmKey)
	if err != nil {
		return LifecycleResult{}, err
	}

	wp, err := s.loadPluginBytes(ctx, pluginID, wasmBytes, config)
	if err != nil {
		return LifecycleResult{}, err
	}

	record := PluginRecord{
		ID:          wp.ID(),
		WasmKey:     wasmKey,
		ConfigJSON:  cloneJSON(config),
		Enabled:     true,
		WasmHash:    hashWASM(wasmBytes),
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.persistPluginState(ctx, record, metadataRecordFromMeta(wp.Meta())); err != nil {
		_ = s.store.DeletePlugin(ctx, wp.ID())
		_ = s.store.DeletePluginMetadata(ctx, wp.ID())
		_ = s.loader.UnloadPlugin(ctx, wp.ID())
		return LifecycleResult{}, err
	}
	if err := s.installStagedFrontend(ctx, wp.ID(), wasmKey); err != nil {
		_ = s.store.DeletePlugin(ctx, wp.ID())
		_ = s.store.DeletePluginMetadata(ctx, wp.ID())
		_ = s.loader.UnloadPlugin(ctx, wp.ID())
		return LifecycleResult{}, err
	}

	s.manager.Register(wp)
	s.registerPluginCommands(wp)
	s.saveVersionBestEffort(ctx, VersionRecord{
		PluginID:   wp.ID(),
		Version:    wp.Version(),
		WasmKey:    wasmKey,
		WasmHash:   record.WasmHash,
		ConfigJSON: cloneJSON(config),
		Changelog:  "initial install",
	})
	s.publish(ctx, pubsub.EventPluginInstalled, wp.ID())

	return LifecycleResult{
		PluginID: wp.ID(),
		Name:     wp.Name(),
		Version:  wp.Version(),
		Status:   "installed",
	}, nil
}

func (s *PluginLifecycleService) Enable(ctx context.Context, pluginID string) (LifecycleResult, error) {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}
	if record.Enabled {
		return LifecycleResult{PluginID: pluginID, Status: "already_enabled"}, nil
	}

	wasmBytes, err := s.readBlob(ctx, record.WasmKey)
	if err != nil {
		return LifecycleResult{}, err
	}

	wp, err := s.loadPluginBytes(ctx, pluginID, wasmBytes, record.ConfigJSON)
	if err != nil {
		return LifecycleResult{}, err
	}

	record.Enabled = true
	record.UpdatedAt = time.Now()
	if err := s.persistPluginState(ctx, record, metadataRecordFromMeta(wp.Meta())); err != nil {
		_ = s.loader.UnloadPlugin(ctx, wp.ID())
		return LifecycleResult{}, err
	}

	s.manager.Register(wp)
	s.registerPluginCommands(wp)
	s.publish(ctx, pubsub.EventPluginEnabled, pluginID)

	return LifecycleResult{
		PluginID: pluginID,
		Name:     wp.Name(),
		Version:  wp.Version(),
		Status:   "enabled",
	}, nil
}

func (s *PluginLifecycleService) Disable(ctx context.Context, pluginID string) (LifecycleResult, error) {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}
	if !record.Enabled {
		return LifecycleResult{PluginID: pluginID, Status: "already_disabled"}, nil
	}

	record.Enabled = false
	record.UpdatedAt = time.Now()
	if err := s.store.SavePlugin(ctx, record); err != nil {
		return LifecycleResult{}, fmt.Errorf("update plugin record: %w", err)
	}

	s.invalidatePluginPolicies(pluginID)
	s.unregisterPluginCommands(pluginID)
	if err := s.loader.UnloadPlugin(ctx, pluginID); err != nil {
		slog.Warn("lifecycle: unload plugin on disable", "plugin", pluginID, "error", err)
	}
	s.manager.Remove(pluginID)
	s.publish(ctx, pubsub.EventPluginDisabled, pluginID)

	return LifecycleResult{PluginID: pluginID, Status: "disabled"}, nil
}

func (s *PluginLifecycleService) Delete(ctx context.Context, pluginID string) (LifecycleResult, error) {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}

	s.invalidatePluginPolicies(pluginID)
	s.unregisterPluginCommands(pluginID)

	mustUnload := record.Enabled
	if !record.Enabled && record.WasmKey != "" {
		if wasmBytes, readErr := s.readBlob(ctx, record.WasmKey); readErr == nil {
			if _, loadErr := s.loader.LoadPluginFromBytes(ctx, wasmBytes, record.ConfigJSON); loadErr != nil {
				slog.Warn("lifecycle: failed to load disabled plugin for cleanup", "plugin", pluginID, "error", loadErr)
			} else {
				mustUnload = true
			}
		}
	}

	if err := s.loader.DropPluginData(ctx, pluginID); err != nil {
		slog.Warn("lifecycle: failed to drop plugin data", "plugin", pluginID, "error", err)
	}
	if s.hostAPI != nil && s.hostAPI.KVStore() != nil {
		s.hostAPI.KVStore().DropPlugin(pluginID)
	}

	if mustUnload {
		if err := s.loader.UnloadPlugin(ctx, pluginID); err != nil {
			slog.Warn("lifecycle: unload plugin during delete", "plugin", pluginID, "error", err)
		}
	}
	s.manager.Remove(pluginID)

	s.cleanupVersionBlobs(ctx, pluginID, record.WasmKey)
	if record.WasmKey != "" {
		if err := s.blobs.Delete(ctx, record.WasmKey); err != nil {
			slog.Warn("lifecycle: delete active wasm blob", "plugin", pluginID, "key", record.WasmKey, "error", err)
		}
		if err := s.blobs.Delete(ctx, pluginFrontendManifestKey(record.WasmKey)); err != nil {
			slog.Warn("lifecycle: delete active frontend manifest", "plugin", pluginID, "key", pluginFrontendManifestKey(record.WasmKey), "error", err)
		}
	}
	s.deletePluginFrontend(ctx, pluginID)

	if s.cmdStore != nil {
		if err := s.cmdStore.DeleteAllPluginCommandSettings(ctx, pluginID); err != nil {
			slog.Error("lifecycle: delete command settings", "plugin", pluginID, "error", err)
		}
	}

	if err := s.store.DeletePlugin(ctx, pluginID); err != nil {
		return LifecycleResult{}, fmt.Errorf("delete plugin record: %w", err)
	}
	if err := s.store.DeletePluginMetadata(ctx, pluginID); err != nil {
		slog.Warn("lifecycle: delete plugin metadata", "plugin", pluginID, "error", err)
	}
	if s.versions != nil {
		if err := s.versions.DeleteVersionsByPlugin(ctx, pluginID); err != nil {
			slog.Warn("lifecycle: delete plugin versions", "plugin", pluginID, "error", err)
		}
	}

	s.publish(ctx, pubsub.EventPluginUninstalled, pluginID)
	return LifecycleResult{PluginID: pluginID, Status: "deleted"}, nil
}

func (s *PluginLifecycleService) Update(ctx context.Context, pluginID string, wasmBytes []byte, changelog string) (LifecycleResult, error) {
	return s.UpdateWithFrontend(ctx, pluginID, wasmBytes, nil, changelog)
}

func (s *PluginLifecycleService) UpdateWithFrontend(ctx context.Context, pluginID string, wasmBytes []byte, frontendFiles []pluginFrontendFile, changelog string) (LifecycleResult, error) {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}

	newKey := fmt.Sprintf("%s_update_%d.wasm", pluginID, time.Now().Unix())
	if err := s.blobs.Put(ctx, newKey, bytesReader(wasmBytes), int64(len(wasmBytes))); err != nil {
		return LifecycleResult{}, fmt.Errorf("save wasm blob: %w", err)
	}

	var stagedFrontend stagedPluginFrontend
	if len(frontendFiles) > 0 {
		stagedFrontend, err = putPluginFrontendAssetsReusing(ctx, s.blobs, pluginID, frontendFiles, s.currentPluginFrontendAssets(ctx, pluginID))
		if err != nil {
			_ = s.blobs.Delete(ctx, newKey)
			return LifecycleResult{}, err
		}
	}

	oldRecord := record
	oldTriggers := s.collectConfigurableTriggers(pluginID)
	oldVersion := s.currentVersion(ctx, pluginID)

	var meta wasmrt.PluginMeta
	meta, err = s.reloadOrProbePlugin(ctx, pluginID, record.Enabled, wasmBytes, record.ConfigJSON, oldTriggers)
	if err != nil {
		_ = s.blobs.Delete(ctx, newKey)
		deleteFrontendAssetsBestEffort(ctx, s.blobs, stagedFrontend.Assets)
		return LifecycleResult{}, err
	}

	record.WasmKey = newKey
	record.WasmHash = hashWASM(wasmBytes)
	record.UpdatedAt = time.Now()
	if err := s.persistPluginState(ctx, record, metadataRecordFromMeta(meta)); err != nil {
		s.rollbackRuntimeIfNeeded(ctx, oldRecord, record.Enabled, oldTriggers)
		_ = s.blobs.Delete(ctx, newKey)
		deleteFrontendAssetsBestEffort(ctx, s.blobs, stagedFrontend.Assets)
		return LifecycleResult{}, err
	}
	if len(frontendFiles) > 0 {
		if err := s.replacePluginFrontend(ctx, pluginID, stagedFrontend); err != nil {
			s.rollbackRuntimeIfNeeded(ctx, oldRecord, record.Enabled, oldTriggers)
			_ = s.blobs.Delete(ctx, newKey)
			deleteFrontendAssetsBestEffort(ctx, s.blobs, stagedFrontend.Assets)
			return LifecycleResult{}, err
		}
	}

	s.saveVersionBestEffort(ctx, VersionRecord{
		PluginID:   pluginID,
		Version:    meta.Version,
		WasmKey:    newKey,
		WasmHash:   record.WasmHash,
		ConfigJSON: cloneJSON(record.ConfigJSON),
		Changelog:  changelog,
	})
	s.publish(ctx, pubsub.EventPluginUpdated, pluginID)

	return LifecycleResult{
		PluginID:   pluginID,
		Version:    meta.Version,
		OldVersion: oldVersion,
		NewVersion: meta.Version,
		Status:     "updated",
	}, nil
}

func (s *PluginLifecycleService) Rollback(ctx context.Context, pluginID string, versionID int64) (LifecycleResult, error) {
	if s.versions == nil {
		return LifecycleResult{}, fmt.Errorf("version store not configured")
	}

	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}
	ver, err := s.versions.GetVersion(ctx, versionID)
	if err != nil {
		return LifecycleResult{}, err
	}
	if ver.PluginID != pluginID {
		return LifecycleResult{}, fmt.Errorf("version %d does not belong to plugin %q", versionID, pluginID)
	}

	wasmBytes, err := s.readBlob(ctx, ver.WasmKey)
	if err != nil {
		return LifecycleResult{}, err
	}
	if got := hashWASM(wasmBytes); got != ver.WasmHash {
		return LifecycleResult{}, fmt.Errorf("wasm binary integrity check failed for %q", ver.WasmKey)
	}

	oldRecord := record
	oldTriggers := s.collectConfigurableTriggers(pluginID)
	oldVersion := s.currentVersion(ctx, pluginID)

	var meta wasmrt.PluginMeta
	meta, err = s.reloadOrProbePlugin(ctx, pluginID, record.Enabled, wasmBytes, ver.ConfigJSON, oldTriggers)
	if err != nil {
		return LifecycleResult{}, err
	}

	record.WasmKey = ver.WasmKey
	record.WasmHash = ver.WasmHash
	record.ConfigJSON = cloneJSON(ver.ConfigJSON)
	record.UpdatedAt = time.Now()
	if err := s.persistPluginState(ctx, record, metadataRecordFromMeta(meta)); err != nil {
		s.rollbackRuntimeIfNeeded(ctx, oldRecord, record.Enabled, oldTriggers)
		return LifecycleResult{}, err
	}

	s.publish(ctx, pubsub.EventPluginUpdated, pluginID)
	return LifecycleResult{
		PluginID:   pluginID,
		Version:    ver.Version,
		OldVersion: oldVersion,
		NewVersion: ver.Version,
		Status:     "rolled_back",
	}, nil
}

func (s *PluginLifecycleService) UpdateConfig(ctx context.Context, pluginID string, config json.RawMessage) (LifecycleResult, error) {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return LifecycleResult{}, err
	}
	if err := s.ValidateConfig(ctx, pluginID, config); err != nil {
		return LifecycleResult{}, err
	}

	oldConfig := cloneJSON(record.ConfigJSON)
	if record.Enabled {
		if err := s.applyConfigToLoadedPlugin(ctx, pluginID, record.WasmKey, config); err != nil {
			return LifecycleResult{}, err
		}
	}

	record.ConfigJSON = cloneJSON(config)
	record.UpdatedAt = time.Now()
	if err := s.store.SavePlugin(ctx, record); err != nil {
		if record.Enabled {
			if rollbackErr := s.applyConfigToLoadedPlugin(ctx, pluginID, record.WasmKey, oldConfig); rollbackErr != nil {
				slog.Error("lifecycle: failed to roll back config after save failure", "plugin", pluginID, "error", rollbackErr)
			}
		}
		return LifecycleResult{}, fmt.Errorf("update plugin record: %w", err)
	}

	s.publish(ctx, pubsub.EventConfigChanged, pluginID)
	return LifecycleResult{PluginID: pluginID, Status: "updated"}, nil
}

func (s *PluginLifecycleService) loadFromStore(ctx context.Context, pluginID string) error {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	if !record.Enabled {
		return nil
	}

	wasmBytes, err := s.readBlob(ctx, record.WasmKey)
	if err != nil {
		return err
	}

	if _, ok := s.loader.GetPlugin(pluginID); ok {
		oldTriggers := s.collectConfigurableTriggers(pluginID)
		if err := s.loader.ReloadPluginFromBytes(ctx, pluginID, wasmBytes, record.ConfigJSON); err != nil {
			return err
		}
		if wp, ok := s.loader.GetPlugin(pluginID); ok {
			s.syncPluginAfterReload(ctx, pluginID, oldTriggers, wp)
		}
		return nil
	}

	wp, err := s.loader.LoadPluginFromBytes(ctx, wasmBytes, record.ConfigJSON)
	if err != nil {
		return err
	}
	s.manager.Register(wp)
	s.registerPluginCommands(wp)
	return nil
}

func (s *PluginLifecycleService) reloadFromStore(ctx context.Context, pluginID string) error {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	if !record.Enabled {
		return nil
	}
	return s.loadFromStore(ctx, pluginID)
}

func (s *PluginLifecycleService) applyStoredConfig(ctx context.Context, pluginID string) error {
	record, err := s.store.GetPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	if !record.Enabled {
		return nil
	}
	if _, ok := s.loader.GetPlugin(pluginID); !ok {
		return nil
	}
	return s.applyConfigToLoadedPlugin(ctx, pluginID, record.WasmKey, record.ConfigJSON)
}

func (s *PluginLifecycleService) applyConfigToLoadedPlugin(ctx context.Context, pluginID, wasmKey string, config json.RawMessage) error {
	wp, ok := s.loader.GetPlugin(pluginID)
	if !ok {
		return fmt.Errorf("plugin %q not loaded", pluginID)
	}
	if s.opts.ReconfigureEnabled && wp.Meta().SupportsReconfigure {
		return s.loader.ReconfigurePlugin(ctx, pluginID, config)
	}

	wasmBytes, err := s.readBlob(ctx, wasmKey)
	if err != nil {
		return err
	}
	oldTriggers := s.collectConfigurableTriggers(pluginID)
	if err := s.loader.ReloadPluginFromBytes(ctx, pluginID, wasmBytes, config); err != nil {
		return err
	}
	next, ok := s.loader.GetPlugin(pluginID)
	if !ok {
		return fmt.Errorf("plugin %q missing after config reload", pluginID)
	}
	s.syncPluginAfterReload(ctx, pluginID, oldTriggers, next)
	return nil
}

func (s *PluginLifecycleService) loadPluginBytes(ctx context.Context, pluginID string, wasmBytes []byte, config json.RawMessage) (lifecyclePlugin, error) {
	wp, err := s.loader.LoadPluginFromBytes(ctx, wasmBytes, config)
	if err != nil {
		return nil, fmt.Errorf("load plugin: %w", err)
	}
	if err := s.ensurePluginID(ctx, pluginID, wp.ID()); err != nil {
		return nil, err
	}
	return wp, nil
}

func (s *PluginLifecycleService) reloadOrProbePlugin(
	ctx context.Context,
	pluginID string,
	enabled bool,
	wasmBytes []byte,
	config json.RawMessage,
	oldTriggers map[string]struct{},
) (wasmrt.PluginMeta, error) {
	if enabled {
		if err := s.loader.ReloadPluginFromBytes(ctx, pluginID, wasmBytes, config); err != nil {
			return wasmrt.PluginMeta{}, fmt.Errorf("reload plugin: %w", err)
		}
		wp, ok := s.loader.GetPlugin(pluginID)
		if !ok {
			return wasmrt.PluginMeta{}, fmt.Errorf("plugin %q missing after reload", pluginID)
		}
		s.syncPluginAfterReload(ctx, pluginID, oldTriggers, wp)
		return wp.Meta(), nil
	}

	meta, err := s.loader.ProbeMetadataFromBytes(ctx, wasmBytes)
	if err != nil {
		return wasmrt.PluginMeta{}, fmt.Errorf("probe plugin metadata: %w", err)
	}
	if err := s.ensurePluginID(ctx, pluginID, meta.ID); err != nil {
		return wasmrt.PluginMeta{}, err
	}
	if err := adapter.ValidateConfigAgainstSchema(meta.ConfigSchema, config); err != nil {
		return wasmrt.PluginMeta{}, err
	}
	return meta, nil
}

func (s *PluginLifecycleService) ensurePluginID(ctx context.Context, wantID, gotID string) error {
	if gotID == wantID {
		return nil
	}
	if gotID != "" {
		_ = s.loader.UnloadPlugin(ctx, gotID)
	}
	return fmt.Errorf("plugin ID mismatch: expected %q, got %q", wantID, gotID)
}

func (s *PluginLifecycleService) removeLocal(ctx context.Context, pluginID string) {
	s.unregisterPluginCommands(pluginID)
	if err := s.loader.UnloadPlugin(ctx, pluginID); err != nil {
		slog.Warn("pubsub: unload plugin", "plugin", pluginID, "error", err)
	}
	s.manager.Remove(pluginID)
}

func (s *PluginLifecycleService) persistPluginState(ctx context.Context, record PluginRecord, meta PluginMetadataRecord) error {
	if err := s.store.SavePlugin(ctx, record); err != nil {
		return fmt.Errorf("save plugin record: %w", err)
	}
	if err := s.store.SavePluginMetadata(ctx, meta); err != nil {
		return fmt.Errorf("save plugin metadata: %w", err)
	}
	return nil
}

func (s *PluginLifecycleService) installStagedFrontend(ctx context.Context, pluginID, wasmKey string) error {
	staged, found, err := readStagedPluginFrontendManifest(ctx, s.blobs, wasmKey)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if staged.PluginID != "" && staged.PluginID != pluginID {
		return fmt.Errorf("frontend plugin ID mismatch: expected %q, got %q", pluginID, staged.PluginID)
	}
	if err := s.replacePluginFrontend(ctx, pluginID, staged); err != nil {
		return err
	}
	if err := s.blobs.Delete(ctx, pluginFrontendManifestKey(wasmKey)); err != nil {
		slog.Warn("lifecycle: delete staged frontend manifest", "plugin", pluginID, "key", pluginFrontendManifestKey(wasmKey), "error", err)
	}
	return nil
}

func (s *PluginLifecycleService) replacePluginFrontend(ctx context.Context, pluginID string, staged stagedPluginFrontend) error {
	frontendStore, ok := pluginFrontendStoreFrom(s.store)
	if !ok {
		return fmt.Errorf("plugin frontend store is not configured")
	}

	var previousAssets []PluginFrontendAsset
	if previous, err := frontendStore.GetPluginFrontend(ctx, pluginID); err == nil {
		previousAssets = previous.Assets
	}

	record := PluginFrontendRecord{
		PluginID:   pluginID,
		Entrypoint: staged.Entrypoint,
		Assets:     staged.Assets,
	}
	if record.Entrypoint == "" {
		record.Entrypoint = pluginFrontendEntrypoint
	}
	if err := frontendStore.SavePluginFrontend(ctx, record); err != nil {
		return err
	}
	deleteUnreferencedFrontendAssetsBestEffort(ctx, s.blobs, previousAssets, record.Assets)
	return nil
}

func (s *PluginLifecycleService) currentPluginFrontendAssets(ctx context.Context, pluginID string) []PluginFrontendAsset {
	frontendStore, ok := pluginFrontendStoreFrom(s.store)
	if !ok {
		return nil
	}
	record, err := frontendStore.GetPluginFrontend(ctx, pluginID)
	if err != nil {
		return nil
	}
	return record.Assets
}

func (s *PluginLifecycleService) deletePluginFrontend(ctx context.Context, pluginID string) {
	frontendStore, ok := pluginFrontendStoreFrom(s.store)
	if !ok {
		return
	}
	record, err := frontendStore.GetPluginFrontend(ctx, pluginID)
	if err == nil {
		deleteFrontendAssetsBestEffort(ctx, s.blobs, record.Assets)
	} else {
		slog.Debug("lifecycle: plugin frontend not found during delete", "plugin", pluginID, "error", err)
	}
	if err := frontendStore.DeletePluginFrontend(ctx, pluginID); err != nil {
		slog.Warn("lifecycle: delete plugin frontend record", "plugin", pluginID, "error", err)
	}
}

func (s *PluginLifecycleService) collectConfigurableTriggers(pluginID string) map[string]struct{} {
	p, ok := s.manager.Get(pluginID)
	if !ok {
		return nil
	}
	if wp, ok := p.(*adapter.WasmPlugin); ok {
		triggers := make(map[string]struct{})
		for _, t := range wp.Meta().Triggers {
			if t.Type != "cron" {
				triggers[t.Name] = struct{}{}
			}
		}
		return triggers
	}
	triggers := make(map[string]struct{}, len(p.Commands()))
	for _, def := range p.Commands() {
		triggers[def.Name] = struct{}{}
	}
	return triggers
}

func (s *PluginLifecycleService) syncPluginAfterReload(ctx context.Context, pluginID string, oldTriggers map[string]struct{}, next plugin.Plugin) {
	s.manager.Remove(pluginID)
	s.manager.Register(next)
	s.registerPluginCommands(next)

	newTriggers := s.collectConfigurableTriggers(pluginID)
	var removed []string
	for name := range oldTriggers {
		if _, ok := newTriggers[name]; !ok {
			removed = append(removed, name)
		}
	}

	if s.stateMgr != nil {
		for _, name := range removed {
			s.stateMgr.UnregisterCommand(pluginID, name)
		}
	}

	if s.cmdStore != nil && len(removed) > 0 {
		if err := s.cmdStore.DeleteCommandSettings(ctx, pluginID, removed); err != nil {
			slog.Error("lifecycle: delete orphaned trigger settings", "plugin", pluginID, "triggers", removed, "error", err)
		}
	}
}

func (s *PluginLifecycleService) registerPluginCommands(p plugin.Plugin) {
	if s.stateMgr == nil {
		return
	}
	for _, def := range p.Commands() {
		s.stateMgr.RegisterCommand(p.ID(), def)
	}
}

func (s *PluginLifecycleService) unregisterPluginCommands(pluginID string) {
	if s.stateMgr == nil {
		return
	}
	if p, ok := s.manager.Get(pluginID); ok {
		for _, def := range p.Commands() {
			s.stateMgr.UnregisterCommand(pluginID, def.Name)
		}
		return
	}
	s.stateMgr.UnregisterAllCommands(pluginID)
}

func (s *PluginLifecycleService) invalidatePluginPolicies(pluginID string) {
	if s.invalidator == nil {
		return
	}
	p, ok := s.manager.Get(pluginID)
	if !ok {
		return
	}
	for _, def := range p.Commands() {
		s.invalidator.InvalidateCommandPolicy(pluginID, def.Name)
	}
}

func (s *PluginLifecycleService) rollbackRuntimeIfNeeded(ctx context.Context, record PluginRecord, enabled bool, oldTriggers map[string]struct{}) {
	if !enabled {
		return
	}
	wasmBytes, err := s.readBlob(ctx, record.WasmKey)
	if err != nil {
		slog.Error("lifecycle: failed to read rollback wasm", "plugin", record.ID, "error", err)
		return
	}
	if err := s.loader.ReloadPluginFromBytes(ctx, record.ID, wasmBytes, record.ConfigJSON); err != nil {
		slog.Error("lifecycle: failed to restore runtime", "plugin", record.ID, "error", err)
		return
	}
	if wp, ok := s.loader.GetPlugin(record.ID); ok {
		s.syncPluginAfterReload(ctx, record.ID, oldTriggers, wp)
	}
}

func (s *PluginLifecycleService) cleanupVersionBlobs(ctx context.Context, pluginID, activeWasmKey string) {
	if s.versions == nil {
		return
	}
	versions, err := s.versions.ListVersions(ctx, pluginID)
	if err != nil {
		slog.Warn("lifecycle: list versions for cleanup", "plugin", pluginID, "error", err)
		return
	}
	for _, v := range versions {
		if v.WasmKey == "" || v.WasmKey == activeWasmKey {
			continue
		}
		if err := s.blobs.Delete(ctx, v.WasmKey); err != nil {
			slog.Warn("lifecycle: delete version wasm blob", "plugin", pluginID, "key", v.WasmKey, "error", err)
		}
	}
}

func (s *PluginLifecycleService) saveVersionBestEffort(ctx context.Context, rec VersionRecord) {
	if s.versions == nil {
		return
	}
	if _, err := s.versions.SaveVersion(ctx, rec); err != nil {
		slog.Error("lifecycle: failed to save version", "plugin", rec.PluginID, "version", rec.Version, "error", err)
	}
}

func (s *PluginLifecycleService) publish(ctx context.Context, eventType, pluginID string) {
	if s.bus == nil {
		return
	}
	if err := s.bus.Publish(ctx, pubsub.AdminEvent{
		Type:     eventType,
		PluginID: pluginID,
	}); err != nil {
		slog.Error("lifecycle: failed to publish event", "type", eventType, "plugin", pluginID, "error", err)
	}
}

func (s *PluginLifecycleService) readBlob(ctx context.Context, key string) ([]byte, error) {
	rc, err := s.blobs.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get blob %q: %w", key, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read blob %q: %w", key, err)
	}
	return data, nil
}

func metadataRecordFromMeta(meta wasmrt.PluginMeta) PluginMetadataRecord {
	metaJSON, _ := json.Marshal(meta)
	requirements, _ := json.Marshal(meta.Requirements)
	triggers, _ := json.Marshal(meta.Triggers)
	return PluginMetadataRecord{
		PluginID:     meta.ID,
		Name:         meta.Name,
		Version:      meta.Version,
		SDKVersion:   meta.SDKVersion,
		MetaJSON:     metaJSON,
		ConfigSchema: cloneJSON(meta.ConfigSchema),
		Requirements: requirements,
		Triggers:     triggers,
		UpdatedAt:    time.Now(),
	}
}

func hashWASM(wasmBytes []byte) string {
	hash := sha256.Sum256(wasmBytes)
	return hex.EncodeToString(hash[:])
}

func cloneJSON(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(v))
	copy(out, v)
	return out
}

func (s *PluginLifecycleService) currentVersion(ctx context.Context, pluginID string) string {
	if p, ok := s.manager.Get(pluginID); ok {
		return p.Version()
	}
	if meta, err := s.store.GetPluginMetadata(ctx, pluginID); err == nil {
		return meta.Version
	}
	return ""
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
