package discord

import (
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
	"context"
	"github.com/bwmarrin/discordgo"
)

func (a *Adapter) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	return a.session.ChannelMessageDelete(chatID, messageID, discordgo.WithContext(ctx))
}

func (a *Adapter) EditMessage(ctx context.Context, chatID, messageID string, msg model.Message) error {
	components := []discordgo.MessageComponent{}
	edit := &discordgo.MessageEdit{ID: messageID, Channel: chatID, Components: &components}
	if !msg.IsEmpty() {
		rendered := a.renderer.Render(msg)
		if len(rendered.FileRefs) > 0 || len(rendered.ImageURLs) > 0 {
			return channel.ErrMessageOperationUnsupported
		}
		edit.Content = &rendered.Text
		for _, row := range rendered.Buttons {
			actions := discordgo.ActionsRow{}
			for _, button := range row {
				actions.Components = append(actions.Components, discordgo.Button{Label: button.Label, CustomID: button.CustomID, Style: discordgo.PrimaryButton})
			}
			components = append(components, actions)
		}
	}
	_, err := a.session.ChannelMessageEditComplex(edit, discordgo.WithContext(ctx))
	return err
}
