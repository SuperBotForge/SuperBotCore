package runtime

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

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

type slogWriter struct {
	pluginID string
}

func (w *slogWriter) Write(p []byte) (int, error) {
	if len(p) > 0 {
		slog.Warn("plugin stderr", "plugin_id", w.pluginID, "output", string(p))
	}
	return len(p), nil
}

var requiredExports = []string{"alloc"}

type CompiledModule struct {
	compiled wazero.CompiledModule
	rt       *Runtime
	pool     *ModulePool
	ID       string
	Version  string
	Hash     string
}

func (r *Runtime) CompileModule(ctx context.Context, wasmBytes []byte) (*CompiledModule, error) {
	compiled, err := r.engine.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile wasm module: %w", err)
	}

	exports := compiled.ExportedFunctions()
	for _, name := range requiredExports {
		if _, ok := exports[name]; !ok {
			_ = compiled.Close(ctx)
			return nil, fmt.Errorf("wasm module missing required export %q", name)
		}
	}

	hash := sha256.Sum256(wasmBytes)

	return &CompiledModule{
		compiled: compiled,
		rt:       r,
		Hash:     hex.EncodeToString(hash[:]),
	}, nil
}

func (cm *CompiledModule) EnablePool(cfg *PoolConfig) {
	if cm.pool != nil {
		cm.pool.Close()
	}
	cm.pool = NewModulePool(cm, cfg)
}

func (cm *CompiledModule) Pool() *ModulePool {
	return cm.pool
}

func (cm *CompiledModule) Close(ctx context.Context) error {
	if cm.pool != nil {
		cm.pool.Close()
		cm.pool = nil
	}
	return cm.compiled.Close(ctx)
}

func (cm *CompiledModule) RunAction(ctx context.Context, action string, input []byte) ([]byte, error) {
	return cm.RunActionWithConfig(ctx, action, input, nil)
}

func (cm *CompiledModule) RunActionWithConfig(ctx context.Context, action string, input []byte, configJSON []byte) ([]byte, error) {
	if cm.pool != nil {
		return cm.pool.Execute(ctx, action, input, configJSON)
	}

	return cm.runActionDirect(ctx, action, input, configJSON)
}

// instantiate creates a fresh WASM module instance, runs it, and returns stdout.
func (cm *CompiledModule) instantiate(ctx context.Context, action string, input, configJSON []byte) ([]byte, error) {
	var stdout bytes.Buffer
	stdin := bytes.NewReader(input)

	var stderr io.Writer = io.Discard
	if cm.ID != "" {
		stderr = &slogWriter{pluginID: cm.ID}
	}

	modCfg := wazero.NewModuleConfig().
		WithEnv("PLUGIN_ACTION", action).
		WithStdin(stdin).
		WithStdout(&stdout).
		WithStderr(stderr).
		WithFSConfig(wazero.NewFSConfig()).
		WithSysWalltime().
		WithName("")

	if len(configJSON) > 0 {
		modCfg = modCfg.WithEnv("PLUGIN_CONFIG", string(configJSON))
	}

	_, err := cm.rt.engine.InstantiateModule(ctx, cm.compiled, modCfg)
	if err != nil {
		if exitErr, ok := err.(*sys.ExitError); ok {
			if exitErr.ExitCode() == 0 {
				return stdout.Bytes(), nil
			}
			return nil, fmt.Errorf("wasm module exited with code %d", exitErr.ExitCode())
		}
		return nil, fmt.Errorf("instantiate wasm module: %w", err)
	}

	return stdout.Bytes(), nil
}

func (cm *CompiledModule) runActionDirect(ctx context.Context, action string, input []byte, configJSON []byte) ([]byte, error) {
	timeoutSec := pluginTimeoutFromContext(ctx, cm.rt.config.DefaultTimeoutSeconds)
	timeout := time.Duration(timeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ctx = context.WithValue(ctx, PluginIDKey{}, cm.ID)

	for _, hook := range cm.rt.contextHooks {
		ctx = hook(ctx, cm.ID)
	}

	start := time.Now()
	status := "ok"
	defer func() {
		duration := time.Since(start)
		if cm.rt.metrics != nil {
			cm.rt.metrics.PluginActionDuration.WithLabelValues(cm.ID, action).Observe(duration.Seconds())
			cm.rt.metrics.PluginActionTotal.WithLabelValues(cm.ID, action, status).Inc()
		}
		slog.Info("plugin action",
			"plugin_id", cm.ID,
			"action", action,
			"duration_ms", duration.Milliseconds(),
			"status", status,
		)
	}()

	result, err := cm.instantiate(ctx, action, input, configJSON)
	if err != nil {
		status = "error"
		return nil, err
	}
	return result, nil
}

func (cm *CompiledModule) CallMeta(ctx context.Context) (PluginMeta, error) {
	data, err := cm.RunAction(ctx, "meta", nil)
	if err != nil {
		return PluginMeta{}, fmt.Errorf("call meta: %w", err)
	}

	var meta PluginMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return PluginMeta{}, fmt.Errorf("unmarshal meta JSON (%q): %w", string(data), err)
	}

	if meta.SDKVersion > MaxSupportedSDKVersion {
		return PluginMeta{}, fmt.Errorf(
			"plugin %q requires SDK protocol v%d, but host supports up to v%d — upgrade the host",
			meta.ID, meta.SDKVersion, MaxSupportedSDKVersion)
	}

	return meta, nil
}

func (cm *CompiledModule) CallConfigure(ctx context.Context, configJSON []byte) error {
	data, err := cm.RunAction(ctx, "configure", configJSON)
	if err != nil {
		return fmt.Errorf("call configure: %w", err)
	}

	if len(data) > 0 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("configure error: %s", errResp.Error)
		}
	}

	return nil
}

func (cm *CompiledModule) CallReconfigure(ctx context.Context, previousConfigJSON, configJSON []byte) error {
	input, err := json.Marshal(ReconfigureRequest{
		PreviousConfig: previousConfigJSON,
		Config:         configJSON,
	})
	if err != nil {
		return fmt.Errorf("marshal reconfigure input: %w", err)
	}

	data, err := cm.RunAction(ctx, ActionReconfigure, input)
	if err != nil {
		return fmt.Errorf("call reconfigure: %w", err)
	}

	if len(data) > 0 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("reconfigure error: %s", errResp.Error)
		}
	}

	return nil
}

func (cm *CompiledModule) CallRPC(ctx context.Context, caller, method string, params []byte, configJSON []byte) ([]byte, error) {
	input, err := json.Marshal(RPCRequest{
		Caller: caller,
		Method: method,
		Params: params,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal rpc input: %w", err)
	}

	data, err := cm.RunActionWithConfig(ctx, ActionHandleRPC, input, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call handle_rpc: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty rpc response")
	}

	var resp RPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal rpc response: %w", err)
	}
	if resp.Status == "error" {
		if resp.Error == "" {
			resp.Error = "rpc call failed"
		}
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Result, nil
}

func (cm *CompiledModule) CallMigrate(ctx context.Context, oldVersion, newVersion string) error {
	input, err := json.Marshal(MigrateRequest{
		OldVersion: oldVersion,
		NewVersion: newVersion,
	})
	if err != nil {
		return fmt.Errorf("marshal migrate input: %w", err)
	}

	data, err := cm.RunAction(ctx, ActionMigrate, input)
	if err != nil {
		return fmt.Errorf("call migrate: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var resp MigrateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	if resp.Status == "error" && resp.Message != "" {
		return fmt.Errorf("migrate error: %s", resp.Message)
	}

	return nil
}

func (cm *CompiledModule) CallStepCallback(ctx context.Context, reqJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "step_callback", reqJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call step_callback: %w", err)
	}
	return data, nil
}

func (cm *CompiledModule) CallHandleEvent(ctx context.Context, eventJSON []byte, configJSON []byte) ([]byte, error) {
	data, err := cm.RunActionWithConfig(ctx, "handle_event", eventJSON, configJSON)
	if err != nil {
		return nil, fmt.Errorf("call handle_event: %w", err)
	}
	return data, nil
}

func (cm *CompiledModule) CallCheckVisibility(ctx context.Context, userID int64) ([]string, error) {
	req, _ := json.Marshal(map[string]int64{"user_id": userID})
	data, err := cm.RunAction(ctx, "check_visibility", req)
	if err != nil {
		return nil, fmt.Errorf("call check_visibility: %w", err)
	}
	var resp struct {
		Visible []string `json:"visible"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal check_visibility response: %w", err)
	}
	return resp.Visible, nil
}
