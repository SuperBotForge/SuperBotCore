package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/samber/do/v2"
)

type Config struct {
	Token           string
	GuildId         string // guild to upload commands, empty for global
	FrontendBaseUrl string
}

func NewDiscordClient(i do.Injector) (*discordgo.Session, error) {
	config := do.MustInvoke[Config](i)
	discord, err := discordgo.New(discordgo.BotToken(config.Token))
	if err != nil {
		return nil, err
	}

	return discord, nil
}
