package adapter

import (
	wasmrt "SuperBotGo/internal/wasm/runtime"
	"testing"
)

func TestAllowBackMetadata(t *testing.T) {
	p := &WasmPlugin{meta: wasmrt.PluginMeta{Triggers: []wasmrt.TriggerDef{
		{Name: "tasks", Type: "messenger", AllowBack: true},
		{Name: "legacy", Type: "messenger"},
	}}}
	commands := p.Commands()
	if !commands[0].AllowBack || commands[1].AllowBack {
		t.Fatal("navigation opt-in was not preserved")
	}
}
