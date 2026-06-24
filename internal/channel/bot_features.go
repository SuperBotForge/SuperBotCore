package channel

import (
	"net/http"

	"SuperBotGo/internal/model"
)

// CommandRegistrar is an optional interface for bots that expose a platform
// command registration step.
type CommandRegistrar interface {
	RegisterCommands(commands []string)
}

// RouteRegistrar is an optional interface for bots that need to mount public
// HTTP routes before the server starts listening.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux) error
}

// CommandEntry describes a single command for the platform's command picker menu.
type CommandEntry struct {
	Name         string            // command name without leading /
	Descriptions map[string]string // locale → description (e.g. "en" → "Join the queue")
}

// CommandHintSetter is an optional interface for bots that can publish command
// hints to the platform's command picker (e.g. Telegram's setMyCommands).
type CommandHintSetter interface {
	SetCommandHints(entries []CommandEntry)
}

// OptionLabel resolves the user-facing label for an option with a safe
// fallback to its value.
func OptionLabel(opt model.Option) string {
	if opt.Label != "" {
		return opt.Label
	}
	return opt.Value
}
