package discord

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/do/v2"
	"q+/internal/discord/interactions"
	"strings"
)

func (b *Bot) uploadCommands(ctx context.Context) error {
	applicationCommands := interactions.BuildApplicationCommands()
	createdCommands, err := b.discord.ApplicationCommandBulkOverwrite(
		b.discord.State.User.ID,
		b.config.GuildId,
		applicationCommands,
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	for _, command := range createdCommands {
		uploadedCommands[command.Name] = command
	}

	return nil
}

var uploadedCommands = map[string]*discordgo.ApplicationCommand{}

func GetUploadedCommandId(name string) string {
	if command, ok := uploadedCommands[name]; ok {
		return command.ID
	}
	return "0"
}

type MentionRenderer struct {
}

func NewMentionRenderer(_ do.Injector) (*MentionRenderer, error) {
	return &MentionRenderer{}, nil
}

func (m *MentionRenderer) ClickableSlashCommand(command string) string {
	firstWord := strings.Split(command, " ")[0]
	return fmt.Sprintf("</%s:%s>", command, GetUploadedCommandId(firstWord))
}
