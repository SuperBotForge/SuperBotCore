package handlers

import (
	"context"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"q+/internal/core"
	"q+/internal/core/discord"
	"q+/internal/discord/oauth"
	"q+/internal/web/jwt"
)

type CommandHandler func(ctx InteractionContext, options OptionMap) error

type AutocompleteHandler func(ctx AutocompleteContext, value string) error

type ComponentHandler func(ctx InteractionContext, entity string, entityId int64) error

type EntryHandler func(ctx InteractionContext) error

type InteractionContext struct {
	S               *discordgo.Session
	I               *discordgo.Interaction
	Options         []*discordgo.ApplicationCommandInteractionDataOption
	Ctx             context.Context
	Core            *core.Core
	FrontendBaseUrl string
	Coder           *jwt.Coder
	Oauth           *oauth.Oauth
	MentionRenderer discord.MentionRenderer
}

type AutocompleteContext struct {
	InteractionContext
}

func useCaseContext[T any](ctx InteractionContext, params T) core.UseCaseContext[T] {
	return core.UseCaseContext[T]{
		Ctx:    ctx.Ctx,
		Core:   ctx.Core,
		Params: params,
	}
}

func (ctx InteractionContext) log() *zerolog.Logger {
	return zerolog.Ctx(ctx.Ctx)
}

func (ctx InteractionContext) serverCommandParams() core.ServerCommandParams {
	return core.ServerCommandParams{
		DiscordServerID:  ctx.I.GuildID,
		DiscordChannelId: ctx.I.ChannelID,
	}
}

func (ctx InteractionContext) interactionResponseEdit(response *discordgo.WebhookEdit) error {
	_, err := ctx.S.InteractionResponseEdit(ctx.I, response, discordgo.WithContext(ctx.Ctx))
	return err
}

func (ctx InteractionContext) interactionCommandRespondCustom(data *discordgo.WebhookParams) error {
	err := ctx.S.InteractionResponseDelete(ctx.I, discordgo.WithContext(ctx.Ctx))
	if err != nil {
		return err
	}

	_, err = ctx.S.FollowupMessageCreate(ctx.I, true, data, discordgo.WithContext(ctx.Ctx))
	return err
}

func (ctx InteractionContext) interactionCommandRespondEphemeral(text string) error {
	return ctx.interactionResponseEdit(editResponse(text))
}

func (ctx InteractionContext) interactionCommandRespond(text string) error {
	return ctx.interactionCommandRespondFollowup(text, true)
}

func (ctx InteractionContext) interactionCommandRespondNoMentions(text string) error {
	return ctx.interactionCommandRespondFollowup(text, false)
}

func (ctx InteractionContext) interactionCommandRespondFollowup(text string, allowMentions bool) error {
	var resp *discordgo.WebhookParams
	if allowMentions {
		resp = followupResponse(text)
	} else {
		resp = followupResponseNoMentions(text)
	}
	return ctx.interactionCommandRespondCustom(resp)
}

func (ctx InteractionContext) interactionRespondSeparateMessage(text string) error {
	err := ctx.S.InteractionResponseDelete(ctx.I, discordgo.WithContext(ctx.Ctx))
	if err != nil {
		return err
	}

	_, err = ctx.S.ChannelMessageSend(ctx.I.ChannelID, text, discordgo.WithContext(ctx.Ctx))
	return err
}

func (ctx InteractionContext) interactionRespond(response *discordgo.InteractionResponse) error {
	return ctx.S.InteractionRespond(ctx.I, response, discordgo.WithContext(ctx.Ctx))
}

func (ctx AutocompleteContext) interactionAutocompleteRespond(choices []*discordgo.ApplicationCommandOptionChoice) error {
	return ctx.interactionRespond(autocompleteResultResponse(choices))
}

func (ctx AutocompleteContext) interactionAutocompleteIntError(message string) error {
	return ctx.interactionRespond(autocompleteIntErrorResponse(message))
}

func (ctx AutocompleteContext) interactionAutocompleteStrError(message string) error {
	return ctx.interactionRespond(autocompleteStrErrorResponse(message))
}

func getMemberName(member *discordgo.Member) (name string) {
	if member.Nick != "" {
		return member.Nick
	} else {
		return getUserName(member.User)
	}
}

func getUserName(user *discordgo.User) (name string) {
	if user.GlobalName != "" {
		return user.GlobalName
	} else {
		return user.Username
	}
}

func memberToUser(member *discordgo.Member) *discord.User {
	if member == nil {
		return nil
	}
	return &discord.User{
		DiscordId: member.User.ID,
		Username:  getMemberName(member),
	}
}

func getUser(i *discordgo.Interaction) *discord.User {
	if i.Member == nil {
		return &discord.User{
			DiscordId: i.User.ID,
			Username:  getUserName(i.User),
		}
	}
	return &discord.User{
		DiscordId: i.Member.User.ID,
		Username:  getMemberName(i.Member),
	}
}
