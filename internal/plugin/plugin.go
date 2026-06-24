package plugin

import (
	"context"

	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
	wasmrt "SuperBotGo/internal/wasm/runtime"
)

type Plugin interface {
	ID() string
	Name() string
	Version() string
	Commands() []*state.CommandDefinition
	HandleEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error)
}

type TriggerProvider interface {
	Triggers() []wasmrt.TriggerDef
}

// VisibilityChecker returns which commands of a plugin are visible to a user.
type VisibilityChecker interface {
	// CheckVisibility returns visible command names for the given user and plugin.
	// Returns nil, false if the plugin does not support visibility checking.
	CheckVisibility(ctx context.Context, userID int64, pluginID string) ([]string, bool)
}

func CommandNames(p Plugin) []string {
	defs := p.Commands()
	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	return names
}
