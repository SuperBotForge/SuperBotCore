package core

import (
	"context"
	"fmt"

	"SuperBotGo/internal/notification"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
)

type Plugin struct {
	api          *plugin.SenderAPI
	tsuLinker    TsuAuthLinker
	dialog       DialogReader
	userService  UserLocaleUpdater
	prefsRepo    notification.PrefsRepository
	pluginLister PluginLister
	authChecker  CommandAuthChecker
	cmdDefs      []*state.CommandDefinition
}

func New(
	api *plugin.SenderAPI,
	tsuLinker TsuAuthLinker,
	dialog DialogReader,
	userService UserLocaleUpdater,
	prefsRepo notification.PrefsRepository,
	pluginLister PluginLister,
	authChecker CommandAuthChecker,
) *Plugin {
	p := &Plugin{
		api:          api,
		tsuLinker:    tsuLinker,
		dialog:       dialog,
		userService:  userService,
		prefsRepo:    prefsRepo,
		pluginLister: pluginLister,
		authChecker:  authChecker,
		cmdDefs: []*state.CommandDefinition{
			StartCommand(),
			LinkCommand(),
			ResumeCommand(),
			SettingsCommand(),
			PluginsCommand(pluginLister, authChecker),
		},
	}
	return p
}

func (p *Plugin) ID() string                           { return "core" }
func (p *Plugin) Name() string                         { return "Core Commands" }
func (p *Plugin) Version() string                      { return "1.0.0" }
func (p *Plugin) Commands() []*state.CommandDefinition { return p.cmdDefs }

func (p *Plugin) HandleEvent(ctx context.Context, event contract.Event) (*contract.EventResponse, error) {
	m, err := event.Messenger()
	if err != nil {
		return nil, fmt.Errorf("core: parse messenger data: %w", err)
	}

	switch m.CommandName {
	case "start":
		return nil, p.handleStart(ctx, m)
	case "link":
		return nil, p.handleLink(ctx, m)
	case "resume":
		return nil, p.handleResume(ctx, m)
	case "settings":
		return nil, p.handleSettings(ctx, m)
	case "plugins":
		return nil, p.handlePlugins(ctx, m)
	default:
		return nil, fmt.Errorf("core: unknown command %q", m.CommandName)
	}
}
