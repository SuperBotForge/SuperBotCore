package handlers

import (
	"q+/internal/core"
)

func Register(ctx InteractionContext, options OptionMap) error {
	ctx.log().Trace().Msg("Register command")

	user, err := core.Register(useCaseContext(ctx, core.RegisterParams{
		ServerCommandParams: ctx.serverCommandParams(),
		DiscordId:           getUser(ctx.I).DiscordId,
		Surname:             options.String("surname"),
		Name:                options.String("name"),
		Patronymic:          options.String("patronymic"),
		Group:               options.String("group"),
		Gmail:               options.String("gmail"),
	}))
	if err != nil {
		return err
	}

	return ctx.interactionCommandRespondEphemeral("Привет, " + core.GetName(user) + "! Теперь можно пользоваться ботом")
}

func CheckNeedForRegistration(ctx InteractionContext, channelTypes core.DiscordChannelType) (bool, error) {
	ctx.log().Trace().Msg("Check registered for registration")

	if !channelInBotChannels(channelTypes) {
		return false, nil
	}

	registered, err := core.IsUserRegistered(useCaseContext(ctx, core.IsUserRegisteredParams{
		UserDiscordId: getUser(ctx.I).DiscordId,
	}))
	if err != nil {
		return false, err
	}

	return !registered, nil
}

func channelInBotChannels(channelTypes core.DiscordChannelType) bool {
	return channelTypes&core.ChannelAll > 0 && channelTypes&^core.ChannelAll == 0
}

func AskForRegister(ctx InteractionContext) error {
	ctx.log().Trace().Msg("Ask for register")

	return core.ErrUserNotRegistered(ctx.MentionRenderer)
}
