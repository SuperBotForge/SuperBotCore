package core

import (
	"context"
	"sort"

	"SuperBotGo/internal/i18n"
	"SuperBotGo/internal/locale"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/plugin"
	"SuperBotGo/internal/plugin/contract"
	"SuperBotGo/internal/state"
)

// PluginLister provides a list of user-facing plugins and their commands.
type PluginLister interface {
	ListUserPlugins(excludeIDs ...string) []plugin.PluginInfo
}

// CommandAuthChecker checks whether a user is allowed to execute a command.
type CommandAuthChecker interface {
	CheckCommand(ctx context.Context, userID model.GlobalUserID, pluginID string, commandName string, requirements *model.RoleRequirements) (bool, error)
}

// hiddenCommands are navigational commands that should not appear in plugin command lists.
var hiddenCommands = map[string]struct{}{
	"start":   {},
	"plugins": {},
}

const (
	pluginListPageSize     = 8
	pluginCommandPageSize  = 7
	pluginParamName        = "plugin"
	pluginCommandParamName = "command"
)

func PluginsCommand(lister PluginLister, authChecker CommandAuthChecker, visibilityChecker plugin.VisibilityChecker) *state.CommandDefinition {
	return state.NewCommand("plugins").
		LocalizedDescription(map[string]string{
			"en": "Browse available plugins",
			"ru": "Обзор плагинов",
		}).
		Description("Browse available plugins").
		Step(pluginParamName, func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.LocalizedText("plugins.title", model.StyleHeader)
				p.LocalizedPaginatedOptions("plugins.choose", pluginListPageSize, func(_ state.StepContext) []model.Option {
					return pluginListOptions(lister)
				})
			})
		}).
		Step(pluginCommandParamName, func(s *state.StepBuilder) {
			s.Prompt(func(p *state.PromptBuilder) {
				p.TextFromContext(func(ctx state.StepContext) string {
					info := findPluginInfo(lister, ctx.Params.Get(pluginParamName))
					if info == nil {
						return i18n.Get("plugins.not_found", ctx.Locale)
					}
					return info.Name
				}, model.StyleHeader)
				p.TextFromContext(func(ctx state.StepContext) string {
					info := findPluginInfo(lister, ctx.Params.Get(pluginParamName))
					if info == nil {
						return ""
					}
					if len(pluginCommandOptions(ctx.Context, ctx.UserID, authChecker, visibilityChecker, *info, ctx.Locale)) == 0 {
						return i18n.Get("plugins.no_commands", ctx.Locale)
					}
					return ""
				}, model.StylePlain)
				p.LocalizedPaginatedOptionsWithProvider("plugins.commands_prompt", pluginCommandPageSize, func(ctx state.StepContext, page int) state.OptionsPage {
					info := findPluginInfo(lister, ctx.Params.Get(pluginParamName))
					var all []model.Option
					if info != nil {
						all = pluginCommandOptions(ctx.Context, ctx.UserID, authChecker, visibilityChecker, *info, ctx.Locale)
					}
					opts, hasMore := pageOptions(all, page, pluginCommandPageSize)
					opts = append(opts, backToPluginsOption(ctx.Locale))
					return state.OptionsPage{
						Options: opts,
						HasMore: hasMore,
					}
				})
			})
		}).
		Build()
}

func pluginListOptions(lister PluginLister) []model.Option {
	plugins := lister.ListUserPlugins()
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	opts := make([]model.Option, 0, len(plugins))
	for _, pl := range plugins {
		if countVisibleCommands(pl) == 0 {
			continue
		}
		opts = append(opts, model.Option{Label: pl.Name, Value: pl.ID})
	}
	return opts
}

func findPluginInfo(lister PluginLister, pluginID string) *plugin.PluginInfo {
	if pluginID == "" {
		return nil
	}
	plugins := lister.ListUserPlugins()
	for i := range plugins {
		if plugins[i].ID == pluginID {
			return &plugins[i]
		}
	}
	return nil
}

func pluginCommandOptions(ctx context.Context, userID model.GlobalUserID, authChecker CommandAuthChecker, visibilityChecker plugin.VisibilityChecker, info plugin.PluginInfo, loc string) []model.Option {
	var visibleSet map[string]struct{}
	if info.SupportsVisibility && visibilityChecker != nil {
		if names, ok := visibilityChecker.CheckVisibility(ctx, int64(userID), info.ID); ok {
			visibleSet = make(map[string]struct{}, len(names))
			for _, n := range names {
				visibleSet[n] = struct{}{}
			}
		}
	}

	options := make([]model.Option, 0, len(info.Commands))
	for _, cmd := range info.Commands {
		if _, hidden := hiddenCommands[cmd.Name]; hidden {
			continue
		}
		if visibleSet != nil {
			if _, visible := visibleSet[cmd.Name]; !visible {
				continue
			}
		}
		if !isCommandAllowed(ctx, userID, authChecker, info.ID, cmd) {
			continue
		}
		fqName := info.ID + "." + cmd.Name
		label := commandMenuLabel(cmd, loc)
		options = append(options, model.Option{Label: label, Value: "/" + fqName})
	}
	return options
}

func countAllowedCommands(ctx context.Context, userID model.GlobalUserID, authChecker CommandAuthChecker, visibilityChecker plugin.VisibilityChecker, p plugin.PluginInfo) int {
	var visibleSet map[string]struct{}
	if p.SupportsVisibility && visibilityChecker != nil {
		if names, ok := visibilityChecker.CheckVisibility(ctx, int64(userID), p.ID); ok {
			visibleSet = make(map[string]struct{}, len(names))
			for _, n := range names {
				visibleSet[n] = struct{}{}
			}
		}
	}

	n := 0
	for _, cmd := range p.Commands {
		if _, hidden := hiddenCommands[cmd.Name]; hidden {
			continue
		}
		if visibleSet != nil {
			if _, visible := visibleSet[cmd.Name]; !visible {
				continue
			}
		}
		if isCommandAllowed(ctx, userID, authChecker, p.ID, cmd) {
			n++
		}
	}
	return n
}

func pageOptions(all []model.Option, page, pageSize int) ([]model.Option, bool) {
	if page < 0 {
		page = 0
	}
	start := page * pageSize
	if start >= len(all) {
		return nil, false
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], end < len(all)
}

func backToPluginsOption(loc string) model.Option {
	return model.Option{
		Label: i18n.Get("plugins.back", loc),
		Value: "/plugins",
	}
}

func countVisibleCommands(p plugin.PluginInfo) int {
	n := 0
	for _, cmd := range p.Commands {
		if _, hidden := hiddenCommands[cmd.Name]; !hidden {
			n++
		}
	}
	return n
}

func (p *Plugin) handlePlugins(ctx context.Context, m *contract.MessengerTriggerData) error {
	pluginID := m.Params.Get(pluginParamName)
	if pluginID == "" {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("plugins.not_found", m.Locale)))
	}

	plugins := p.pluginLister.ListUserPlugins()
	var info *plugin.PluginInfo
	for i := range plugins {
		if plugins[i].ID == pluginID {
			info = &plugins[i]
			break
		}
	}
	if info == nil {
		return p.api.Reply(ctx, m, model.NewTextMessage(i18n.Get("plugins.not_found", m.Locale)))
	}

	options := pluginCommandOptions(ctx, m.UserID, p.authChecker, p.visibilityChecker, *info, m.Locale)

	if len(options) == 0 {
		return p.api.Reply(ctx, m, model.Message{
			Blocks: []model.ContentBlock{
				model.TextBlock{Text: info.Name, Style: model.StyleHeader},
				model.TextBlock{Text: i18n.Get("plugins.no_commands", m.Locale), Style: model.StylePlain},
				model.OptionsBlock{
					Options: []model.Option{
						backToPluginsOption(m.Locale),
					},
				},
			},
		})
	}

	// "Back" button
	options = append(options, backToPluginsOption(m.Locale))

	return p.api.Reply(ctx, m, model.Message{
		Blocks: []model.ContentBlock{
			model.TextBlock{
				Text:  info.Name,
				Style: model.StyleHeader,
			},
			model.OptionsBlock{
				Prompt:  i18n.Get("plugins.commands_prompt", m.Locale),
				Options: options,
			},
		},
	})
}

func (p *Plugin) isCommandAllowed(ctx context.Context, userID model.GlobalUserID, pluginID string, cmd plugin.PluginCommand) bool {
	return isCommandAllowed(ctx, userID, p.authChecker, pluginID, cmd)
}

func isCommandAllowed(ctx context.Context, userID model.GlobalUserID, authChecker CommandAuthChecker, pluginID string, cmd plugin.PluginCommand) bool {
	if authChecker == nil {
		return true
	}
	ok, err := authChecker.CheckCommand(ctx, userID, pluginID, cmd.Name, cmd.Requirements)
	if err != nil {
		return false
	}
	return ok
}

func commandMenuLabel(cmd plugin.PluginCommand, loc string) string {
	if label := locale.ResolveText(cmd.Descriptions, loc); label != "" {
		return label
	}
	if cmd.Description != "" {
		return cmd.Description
	}
	return cmd.Name
}
