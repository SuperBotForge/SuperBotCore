package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
	"slices"
)
import "github.com/samber/lo"

var AllCommandBuilders = slices.Concat(
	channelsForCourseCommands,
	courseInstanceCommands,
	criterionCommands,
	queueCommands,
	queueTemplateCommands,
	webCommands,
	markCommands,
	userCommands,
	serverCommands,
)

type builder interface {
	ApplicationCommand() *discordgo.ApplicationCommand
	ApplicationCommandOption() *discordgo.ApplicationCommandOption
	ApplicationCommandName() string
	Handler() handlers.CommandHandler
	Children() []builder
	AutocompleteOptions() []autocompleteBuilder
	ChannelType() core.DiscordChannelType
	Permission() core.DiscordCommandPermission
}

type autocompleteBuilder interface {
	ApplicationCommandOption() *discordgo.ApplicationCommandOption
	AutocompleteHandler() handlers.AutocompleteHandler
}

type applicationCommandWrapper struct {
	applicationCommand *discordgo.ApplicationCommand
	channelType        core.DiscordChannelType
	children           []builder
	permission         core.DiscordCommandPermission
}

func (a *applicationCommandWrapper) ApplicationCommand() *discordgo.ApplicationCommand {
	return a.applicationCommand
}

func (a *applicationCommandWrapper) ApplicationCommandOption() *discordgo.ApplicationCommandOption {
	return nil
}

func (a *applicationCommandWrapper) ApplicationCommandName() string {
	return a.applicationCommand.Name
}

func (a *applicationCommandWrapper) Handler() handlers.CommandHandler {
	return nil
}

func (a *applicationCommandWrapper) Children() []builder {
	return a.children
}

func (a *applicationCommandWrapper) AutocompleteOptions() []autocompleteBuilder {
	return nil
}

func (a *applicationCommandWrapper) ChannelType() core.DiscordChannelType {
	return a.channelType
}

func (a *applicationCommandWrapper) Permission() core.DiscordCommandPermission {
	return a.permission
}

type applicationCommandWithHandler struct {
	applicationCommand  *discordgo.ApplicationCommand
	channelType         core.DiscordChannelType
	handler             handlers.CommandHandler
	autocompleteOptions []*applicationOptionWithAutocomplete
	permission          core.DiscordCommandPermission
}

func (a *applicationCommandWithHandler) ApplicationCommand() *discordgo.ApplicationCommand {
	return a.applicationCommand
}

func (a *applicationCommandWithHandler) ApplicationCommandOption() *discordgo.ApplicationCommandOption {
	return nil
}

func (a *applicationCommandWithHandler) ApplicationCommandName() string {
	return a.applicationCommand.Name
}

func (a *applicationCommandWithHandler) Handler() handlers.CommandHandler {
	return a.handler
}

func (a *applicationCommandWithHandler) Children() []builder {
	return nil
}

func (a *applicationCommandWithHandler) AutocompleteOptions() []autocompleteBuilder {
	return lo.Map(a.autocompleteOptions, func(option *applicationOptionWithAutocomplete, _ int) autocompleteBuilder {
		return option
	})
}

func (a *applicationCommandWithHandler) ChannelType() core.DiscordChannelType {
	return a.channelType
}

func (a *applicationCommandWithHandler) Permission() core.DiscordCommandPermission {
	return a.permission
}

type applicationOptionWithHandler struct {
	option              *discordgo.ApplicationCommandOption
	handler             handlers.CommandHandler
	channelType         core.DiscordChannelType
	children            []*applicationOptionWithHandler
	autocompleteOptions []*applicationOptionWithAutocomplete
}

func (a *applicationOptionWithHandler) ApplicationCommand() *discordgo.ApplicationCommand {
	return nil
}

func (a *applicationOptionWithHandler) ApplicationCommandOption() *discordgo.ApplicationCommandOption {
	return a.option
}

func (a *applicationOptionWithHandler) ApplicationCommandName() string {
	return a.option.Name
}

func (a *applicationOptionWithHandler) Handler() handlers.CommandHandler {
	return a.handler
}

func (a *applicationOptionWithHandler) Children() []builder {
	return lo.Map(a.children, func(child *applicationOptionWithHandler, _ int) builder {
		return child
	})
}

func (a *applicationOptionWithHandler) AutocompleteOptions() []autocompleteBuilder {
	return lo.Map(a.autocompleteOptions, func(option *applicationOptionWithAutocomplete, _ int) autocompleteBuilder {
		return option
	})
}

func (a *applicationOptionWithHandler) ChannelType() core.DiscordChannelType {
	return a.channelType
}

func (a *applicationOptionWithHandler) Permission() core.DiscordCommandPermission {
	return core.PermissionAnyone
}

type applicationOptionWithAutocomplete struct {
	option              *discordgo.ApplicationCommandOption
	autocompleteHandler handlers.AutocompleteHandler
}

func (a *applicationOptionWithAutocomplete) ApplicationCommandOption() *discordgo.ApplicationCommandOption {
	return a.option
}

func (a *applicationOptionWithAutocomplete) AutocompleteHandler() handlers.AutocompleteHandler {
	return a.autocompleteHandler
}

func command(channelType core.DiscordChannelType, command *discordgo.ApplicationCommand, subcommands ...*applicationOptionWithHandler) *applicationCommandWrapper {
	command.Options = lo.Map(subcommands, func(subcommand *applicationOptionWithHandler, _ int) *discordgo.ApplicationCommandOption {
		return subcommand.option
	})
	permission := core.PermissionAdmin
	if channelType != core.ChannelStudent {
		command.DefaultMemberPermissions = lo.ToPtr(int64(discordgo.PermissionManageRoles))
		permission = core.PermissionExaminer
	} else {
		permission = core.PermissionStudent
	}
	return &applicationCommandWrapper{
		applicationCommand: command,
		channelType:        channelType,
		children: lo.Map(subcommands, func(subcommand *applicationOptionWithHandler, _ int) builder {
			// TODO refactor
			subcommand.channelType = channelType
			for _, child := range subcommand.children {
				child.channelType = channelType
			}
			return subcommand
		}),
		permission: permission,
	}
}

func commandF(channelType core.DiscordChannelType, handler handlers.CommandHandler, command *discordgo.ApplicationCommand, options ...*applicationOptionWithAutocomplete) *applicationCommandWithHandler {
	command.Options = slices.Concat(command.Options, lo.Map(options, func(option *applicationOptionWithAutocomplete, _ int) *discordgo.ApplicationCommandOption {
		return option.option
	}))
	requiredOptions := lo.Filter(command.Options, func(option *discordgo.ApplicationCommandOption, _ int) bool {
		return option.Required
	})
	nonRequiredOptions := lo.Filter(command.Options, func(option *discordgo.ApplicationCommandOption, _ int) bool {
		return !option.Required
	})
	command.Options = slices.Concat(requiredOptions, nonRequiredOptions)

	permission := core.PermissionAdmin
	if channelType != core.ChannelStudent {
		command.DefaultMemberPermissions = lo.ToPtr(int64(discordgo.PermissionManageRoles))
		permission = core.PermissionExaminer
	} else {
		permission = core.PermissionStudent
	}

	return &applicationCommandWithHandler{
		applicationCommand:  command,
		channelType:         channelType,
		handler:             handler,
		autocompleteOptions: options,
		permission:          permission,
	}
}

func commandForAnyone(channelType core.DiscordChannelType, handler handlers.CommandHandler, command *discordgo.ApplicationCommand, options ...*applicationOptionWithAutocomplete) *applicationCommandWithHandler {
	c := commandF(channelType, handler, command, options...)
	c.applicationCommand.DefaultMemberPermissions = nil
	c.permission = core.PermissionAnyone
	return c
}

func commandForAdmin(channelType core.DiscordChannelType, handler handlers.CommandHandler, command *discordgo.ApplicationCommand, options ...*applicationOptionWithAutocomplete) *applicationCommandWithHandler {
	c := commandF(channelType, handler, command, options...)
	c.applicationCommand.DefaultMemberPermissions = lo.ToPtr(int64(discordgo.PermissionAdministrator))
	c.permission = core.PermissionAdmin
	return c
}

func subcommandF(handler handlers.CommandHandler, option *discordgo.ApplicationCommandOption, options ...*applicationOptionWithAutocomplete) *applicationOptionWithHandler {
	option.Type = discordgo.ApplicationCommandOptionSubCommand
	option.Options = slices.Concat(option.Options, lo.Map(options, func(option *applicationOptionWithAutocomplete, _ int) *discordgo.ApplicationCommandOption {
		return option.option
	}))
	requiredOptions := lo.Filter(option.Options, func(option *discordgo.ApplicationCommandOption, _ int) bool {
		return option.Required
	})
	nonRequiredOptions := lo.Filter(option.Options, func(option *discordgo.ApplicationCommandOption, _ int) bool {
		return !option.Required
	})
	option.Options = slices.Concat(requiredOptions, nonRequiredOptions)
	return &applicationOptionWithHandler{
		option:              option,
		handler:             handler,
		children:            nil,
		autocompleteOptions: options,
	}
}

func subcommandGroup(group *discordgo.ApplicationCommandOption, subcommands ...*applicationOptionWithHandler) *applicationOptionWithHandler {
	group.Type = discordgo.ApplicationCommandOptionSubCommandGroup
	group.Options = lo.Map(subcommands, func(subcommand *applicationOptionWithHandler, _ int) *discordgo.ApplicationCommandOption {
		return subcommand.option
	})
	return &applicationOptionWithHandler{
		option:   group,
		handler:  nil,
		children: subcommands,
	}
}

func option(option *discordgo.ApplicationCommandOption) *applicationOptionWithAutocomplete {
	return &applicationOptionWithAutocomplete{
		option:              option,
		autocompleteHandler: nil,
	}
}

func autocompleteOption(handler handlers.AutocompleteHandler, option *discordgo.ApplicationCommandOption) *applicationOptionWithAutocomplete {
	option.Autocomplete = true
	return &applicationOptionWithAutocomplete{
		option:              option,
		autocompleteHandler: handler,
	}
}

func BuildApplicationCommands() []*discordgo.ApplicationCommand {
	return buildApplicationCommandStructure(AllCommandBuilders)
}

func BuildCommandsHandler() handlers.EntryHandler {
	return buildHandler(AllCommandBuilders)
}

// buildApplicationCommandStructure is a recursive function that builds a list of application commands
func buildApplicationCommandStructure(builders []builder) []*discordgo.ApplicationCommand {
	commands := make([]*discordgo.ApplicationCommand, 0)
	for _, builder := range builders {
		if builder.ApplicationCommand() != nil {
			commands = append(commands, builder.ApplicationCommand())
		}
	}
	return commands
}

// buildHandler is a recursive function that builds a function that calls the handler for the command or pass the call to the children handler
func buildHandler(commands []builder) handlers.EntryHandler {
	return func(ctx handlers.InteractionContext) error {
		for _, command := range commands {
			var (
				name    string
				options []*discordgo.ApplicationCommandInteractionDataOption
			)
			if command.ApplicationCommand() != nil {
				name = ctx.I.ApplicationCommandData().Name
				options = ctx.I.ApplicationCommandData().Options
			} else if command.ApplicationCommandOption() != nil {
				name = ctx.Options[0].Name
				options = ctx.Options[0].Options
			} else {
				continue
			}
			if command.ApplicationCommandName() == name {
				ctx.Options = options
				if ctx.I.Type == discordgo.InteractionApplicationCommand {
					optionMap := lo.Associate(options, func(option *discordgo.ApplicationCommandInteractionDataOption) (string, *discordgo.ApplicationCommandInteractionDataOption) {
						return option.Name, option
					})
					if command.Handler() != nil {
						err := handlers.CheckChannelType(ctx, command.ChannelType())
						if err != nil {
							return err
						}
						//needForRegistration, err := handlers.CheckNeedForRegistration(ctx, command.ChannelType())
						//if err != nil {
						//	return err
						//}
						//if needForRegistration {
						//	return handlers.AskForRegister(ctx)
						//}

						return command.Handler()(ctx, optionMap)
					}
				} else if ctx.I.Type == discordgo.InteractionApplicationCommandAutocomplete {
					if len(command.AutocompleteOptions()) > 0 {
						for _, receivedOption := range ctx.Options {
							if receivedOption.Focused {
								for _, availableOption := range command.AutocompleteOptions() {
									if receivedOption.Name == availableOption.ApplicationCommandOption().Name {
										err := handlers.CheckChannelType(ctx, command.ChannelType())
										if err != nil {
											return err
										}
										autocompleteCtx := handlers.AutocompleteContext{InteractionContext: ctx}
										return availableOption.AutocompleteHandler()(autocompleteCtx, receivedOption.Value.(string))
									}
								}
							}
						}
					}
				}
				if len(command.Children()) > 0 {
					return buildHandler(command.Children())(ctx)
				}
			}
		}
		return nil
	}
}
