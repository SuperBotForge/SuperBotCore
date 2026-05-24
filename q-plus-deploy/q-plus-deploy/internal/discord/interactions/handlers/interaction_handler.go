package handlers

import (
	"context"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/core/discord"
	"q+/internal/discord/oauth"
	"q+/internal/ent/rule"
	"q+/internal/utils"
	"q+/internal/web/jwt"
	"runtime/debug"
	"strings"
	"unicode/utf8"
)

func CreateEntryHandlerWrapper(
	ctx context.Context,
	c *core.Core,
	frontBaseUrl string,
	coder *jwt.Coder,
	oauth *oauth.Oauth,
	mentionRenderer discord.MentionRenderer,
	entryCommandHandler EntryHandler,
	entryComponentHandler EntryHandler,
) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		logger := zerolog.Ctx(ctx).With().
			Str("interaction_id", i.ID).
			Str("guild_id", i.GuildID).
			Str("channel_id", i.ChannelID).
			Logger()
		defer func() {
			if r := recover(); r != nil {
				logger.Error().
					Str("event", "panic_handling_interaction").
					Str("error", fmt.Sprintf("%v", r)).
					Str("stacktrace", string(debug.Stack())).
					Msg("Panic handling interaction")
				debug.PrintStack()

				errorMsg := fmt.Sprintf("[panic] 🔧 Error: %s", r)
				interactionId := fmt.Sprintf("\ninteraction_id: %s", i.ID)
				var err error
				if i.Type == discordgo.InteractionApplicationCommand {
					_, err = s.InteractionResponseEdit(i.Interaction, editResponse(errorMsg+interactionId), discordgo.WithContext(ctx))
				} else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
					interactionId = strings.ReplaceAll(interactionId, "\n", " ")
					errorMsg = utils.LimitString(errorMsg, 100-utf8.RuneCountInString(interactionId)) + interactionId

					// TODO лютый костыль
					err1 := s.InteractionRespond(i.Interaction, autocompleteStrErrorResponse(errorMsg), discordgo.WithContext(ctx))
					err2 := s.InteractionRespond(i.Interaction, autocompleteIntErrorResponse(errorMsg), discordgo.WithContext(ctx))
					err = errors.Join(err1, err2)
				}
				if err != nil {
					logger.Error().
						Str("event", "error_responding_to_interaction_panic").
						Err(err).
						Msg("Error responding to interaction panic")
				}
			}
		}()

		var user *discordgo.User
		if i.User != nil {
			user = i.User
		} else if i.Member != nil {
			user = i.Member.User
		}
		if user != nil {
			logger = logger.With().
				Str("user_id", user.ID).
				Logger()
		}

		logger.Info().
			Str("event", "interaction_received").
			Str("interaction_type", i.Type.String()).
			Msg("Interaction received")

		// TODO logger command full name

		ctx = logger.WithContext(ctx)
		ctx = context.WithValue(ctx, rule.DiscordRuleCtxKey, true)

		interactionContext := InteractionContext{
			S:               s,
			I:               i.Interaction,
			Ctx:             ctx,
			Core:            c,
			FrontendBaseUrl: frontBaseUrl,
			Coder:           coder,
			Oauth:           oauth,
			MentionRenderer: mentionRenderer,
		}

		// TODO хочется все таки прям вообще все в транзакцию заворачивать, чтобы если даже вывести результат пользователю не получилось, то транзакция откатилась
		var err error
		switch i.Type {
		case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
			interactionContext.Options = i.ApplicationCommandData().Options

			if i.Type == discordgo.InteractionApplicationCommand {
				err := s.InteractionRespond(i.Interaction, deferredEphemeralResponse(), discordgo.WithContext(ctx))
				if err != nil {
					logger.Error().
						Str("event", "error_responding_deferred").
						Err(err).
						Msg("Error responding deferred")
				}
				logger.Trace().
					Str("event", "deferred_responded").
					Msg("Deferred responded")
			}
			err = entryCommandHandler(interactionContext)
		case discordgo.InteractionMessageComponent:
			err = entryComponentHandler(interactionContext)
			customId := strings.Split(i.MessageComponentData().CustomID, "_")[0]
			logger.Trace().
				Str("event", "interaction_message_component").
				Str("custom_id", customId).
				Msg("Interaction message component")
		case discordgo.InteractionModalSubmit:
			err = entryComponentHandler(interactionContext)
			customId := strings.Split(i.ModalSubmitData().CustomID, "_")[0]
			logger.Trace().
				Str("event", "interaction_modal_submit").
				Str("custom_id", customId).
				Msg("Interaction modal submit")
		}

		if err != nil {
			if constraintErr, ok := lo.ErrorsAs[*pgconn.PgError](err); ok { // TODO dependence on pg
				err = fmt.Errorf("pg_error: %s: %w", constraintErr.Detail, err)
			}

			logger.Error().
				Str("event", "error_handling_interaction").
				Err(err).
				Msg("Error handling interaction")

			errorMsg := ""
			interactionId := ""
			if commandErr, ok := lo.ErrorsAs[*core.HumanReadableError](err); ok {
				errorMsg = commandErr.UserError
			} else {
				errorMsg = fmt.Sprintf("[error] 🔧 Error: %s", err.Error())
				interactionId = fmt.Sprintf("\ninteraction_id: %s", i.ID)
			}
			err = nil
			if i.Type == discordgo.InteractionApplicationCommand {
				errorMsg = errorMsg + interactionId
				err = interactionContext.interactionCommandRespondEphemeral(errorMsg)
			} else if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
				autoCtx := AutocompleteContext{InteractionContext: interactionContext}

				interactionId = strings.ReplaceAll(interactionId, "\n", " ")
				errorMsg = utils.LimitString(errorMsg, 100-utf8.RuneCountInString(interactionId)) + interactionId

				// TODO лютый костыль
				err1 := autoCtx.interactionAutocompleteIntError(errorMsg)
				err2 := autoCtx.interactionAutocompleteStrError(errorMsg)
				err = errors.Join(err1, err2)
			}
			if err != nil {
				logger.Error().
					Str("event", "error_responding_to_interaction_error").
					Err(err).
					Msg("Error responding to interaction error")
			}
		}
		logger.Info().
			Str("event", "interaction_handled").
			Msg("Interaction handled")
	}
}
