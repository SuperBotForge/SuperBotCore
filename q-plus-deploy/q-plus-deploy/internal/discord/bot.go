package discord

import (
	"context"
	"errors"
	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog"
	"github.com/samber/do/v2"
	"q+/internal/core"
	coreDiscord "q+/internal/core/discord"
	"q+/internal/discord/interactions"
	"q+/internal/discord/interactions/handlers"
	"q+/internal/discord/oauth"
	"q+/internal/web/jwt"
	"sync"
)

type Bot struct {
	discord         *discordgo.Session
	config          Config
	core            *core.Core
	coder           *jwt.Coder
	oauth           *oauth.Oauth
	mentionRenderer coreDiscord.MentionRenderer
}

func NewBot(i do.Injector) (*Bot, error) {
	config := do.MustInvoke[Config](i)
	discord := do.MustInvoke[*discordgo.Session](i)
	c := do.MustInvoke[*core.Core](i)
	coder := do.MustInvoke[*jwt.Coder](i)
	o := do.MustInvoke[*oauth.Oauth](i)
	mentionRenderer := do.MustInvokeAs[coreDiscord.MentionRenderer](i)

	return &Bot{
		discord:         discord,
		config:          config,
		core:            c,
		coder:           coder,
		oauth:           o,
		mentionRenderer: mentionRenderer,
	}, nil
}

func (b *Bot) StartBot(ctx context.Context) (err error) {
	log := zerolog.Ctx(ctx)

	wg := sync.WaitGroup{}
	var once sync.Once
	wg.Add(1)
	b.discord.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Info().
			Str("event", "logged_in").
			Str("username", s.State.User.Username).
			Str("discriminator", s.State.User.Discriminator).
			Str("id", s.State.User.ID).
			Msgf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
		once.Do(func() {
			wg.Done()
		})
	})
	b.registerCommandHandler(ctx)

	err = b.discord.Open()
	if err != nil {
		return err
	}
	defer func() {
		log.Info().
			Str("event", "closing_discord").
			Msg("Closing discordgo session...")
		closeErr := b.discord.Close()
		err = errors.Join(err, closeErr)
	}()

	log.Info().
		Str("event", "waiting_ready").
		Msg("Waiting for READY event...")
	wg.Wait()

	err = b.uploadCommands(ctx)
	if err != nil {
		return err
	}
	log.Info().
		Str("event", "bot_ready").
		Msg("Bot is ready")

	<-ctx.Done()
	log.Info().
		Str("event", "shutting_down").
		Msg("Bot is shutting down")
	return nil
}

func (b *Bot) registerCommandHandler(ctx context.Context) {
	entryCommandHandler := interactions.BuildCommandsHandler()
	entryComponentHandler := interactions.BuildComponentsHandler()
	b.discord.AddHandler(handlers.CreateEntryHandlerWrapper(
		ctx,
		b.core,
		b.config.FrontendBaseUrl,
		b.coder,
		b.oauth,
		b.mentionRenderer,
		entryCommandHandler,
		entryComponentHandler,
	))
}
