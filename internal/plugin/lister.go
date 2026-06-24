package plugin

import "SuperBotGo/internal/model"

// PluginInfo describes a plugin for user-facing display.
type PluginInfo struct {
	ID                 string
	Name               string
	Commands           []PluginCommand
	SupportsVisibility bool
}

// PluginCommand describes a single command within a plugin.
type PluginCommand struct {
	Name         string
	Descriptions map[string]string
	Description  string                  // Deprecated: use Descriptions for user-facing command text.
	Requirements *model.RoleRequirements // nil = no restriction
}
