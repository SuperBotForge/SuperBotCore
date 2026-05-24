package handlers

import "github.com/bwmarrin/discordgo"

func basicResponse(message string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	}
}

func ephemeralResponse(message string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}
}

func deferredResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
}

func deferredEphemeralResponse() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
}

func editResponse(message string) *discordgo.WebhookEdit {
	return &discordgo.WebhookEdit{
		Content: &message,
	}
}

func autocompleteResultResponse(choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}

}

func autocompleteIntErrorResponse(message string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  message,
					Value: 0,
				},
			},
		},
	}
}

func autocompleteStrErrorResponse(message string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{
					Name:  message,
					Value: "",
				},
			},
		},
	}
}

func followupResponse(message string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: message,
	}
}

func followupResponseNoMentions(message string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content:         message,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func followupEphemeralResponse(message string) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content: message,
		Flags:   discordgo.MessageFlagsEphemeral,
	}
}
