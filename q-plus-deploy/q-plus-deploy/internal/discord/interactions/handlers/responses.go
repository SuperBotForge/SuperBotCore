package handlers

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/core"
	"q+/internal/discord/interactions/components"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strconv"
	"strings"
)

func criteriaWizardDeferredResponse(course *ent.CourseInstance) *discordgo.WebhookEdit {
	return &discordgo.WebhookEdit{
		Embeds:          lo.ToPtr(criteriaWizardEmbeds(course)),
		Components:      lo.ToPtr(components.CriteriaWizardButtons("course", course.ID)),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func criteriaWizardAfterEditingResponse(response *core.EditCriteriaResponse) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:          utils.Prepend(criteriaWizardEmbeds(response.Course), criteriaWizardEditingInfoEmbed(response)),
			Components:      components.CriteriaWizardButtons("course", response.Course.ID),
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	}
}

func criteriaWizardEditingInfoEmbed(response *core.EditCriteriaResponse) *discordgo.MessageEmbed {
	var builder strings.Builder

	if len(response.Created) == 0 && len(response.Updated) == 0 && len(response.Deleted) == 0 {
		builder.WriteString("Нет изменений")
	} else {
		if len(response.Created) > 0 {
			builder.WriteString("➕ Созданы ")
			builder.WriteString(strconv.Itoa(len(response.Created)))
			builder.WriteString(":\n")
			for _, criterion := range response.Created {
				builder.WriteString("- ")
				builder.WriteString(criterion)
				builder.WriteRune('\n')
			}
		}
		if len(response.Updated) > 0 {
			builder.WriteString("✏ Изменены ")
			builder.WriteString(strconv.Itoa(len(response.Updated)))
			builder.WriteString(":\n")
			for _, criterion := range response.Updated {
				builder.WriteString("- ")
				builder.WriteString(criterion.A)
				builder.WriteString(" -> ")
				builder.WriteString(criterion.B)
				builder.WriteRune('\n')
			}
		}
		if len(response.Deleted) > 0 {
			builder.WriteString("🗑 Удалены ")
			builder.WriteString(strconv.Itoa(len(response.Deleted)))
			builder.WriteString(":\n")
			for _, criterion := range response.Deleted {
				builder.WriteString("- ")
				builder.WriteString(criterion)
				builder.WriteRune('\n')
			}
		}
	}

	return &discordgo.MessageEmbed{
		Title:       "Изменения в критериях",
		Description: builder.String(),
	}
}

func createCriteriaList(course *ent.CourseInstance) string {
	var builder strings.Builder

	if len(course.Edges.Criteria) == 0 {
		builder.WriteString("пусто")
	} else {
		builder.WriteString("```\n")
		for i, criterion := range course.Edges.Criteria {
			builder.WriteString(strconv.Itoa(i + 1))
			builder.WriteString(". ")
			builder.WriteString(criterion.Name)
			builder.WriteRune('\n')
		}
		builder.WriteString("```")
	}

	return builder.String()
}

func criteriaWizardEmbeds(course *ent.CourseInstance) []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{
		{
			Title:       utils.LimitString("Критерии в предмете '"+course.Name, 255) + "'", // limit 256 chars https://discord.com/developers/docs/resources/channel#embed-object-embed-limits
			Description: createCriteriaList(course),
			Footer: &discordgo.MessageEmbedFooter{
				Text: "мяу",
			},
		},
	}
}

func editCriteriaModal(course *ent.CourseInstance, criteriaList string) *discordgo.InteractionResponse {
	title := utils.LimitString("Критерии предмета '"+course.Name, 44) + "'" // limit 45 chars https://discord.com/developers/docs/interactions/receiving-and-responding#interaction-response-object-modal
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: components.FillCriteriaEditModal("course", course.ID, title, criteriaList),
	}
}

func queueTeacherSetSelectMenuResponse(response *core.SetTeacherResponse) *discordgo.WebhookParams {
	note := ""
	if len(response.Examiner.Note) > 0 {
		note = " с заметкой '" + response.Examiner.Note + "'"
	}
	return &discordgo.WebhookParams{
		Content:         fmt.Sprintf("Выбор критериев для принимающего <@%s>%s", response.Examiner.Edges.Teacher.DiscordID, note),
		Components:      components.ChooseCriteriaSelectMenu("examiner", response.Examiner.ID, response.Criteria, nil), // TODO selectedCriteria
		AllowedMentions: &discordgo.MessageAllowedMentions{},
		Flags:           discordgo.MessageFlagsEphemeral,
	}
}

func signUpSelectMenuResponse(response *core.SignUpResponse) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content:         fmt.Sprintf("Выбор критериев для сдачи в очереди %s", response.QueuePlace.Edges.Queue.Name),
		Components:      components.ChooseCriteriaSelectMenu("queue-place", response.QueuePlace.ID, response.Criteria, nil), // TODO selectedCriteria
		AllowedMentions: &discordgo.MessageAllowedMentions{},
		Flags:           discordgo.MessageFlagsEphemeral,
	}
}

func queueTemplateCriteriaSelectMenuResponse(response *core.SelectCriteriaForQueueTemplateResponse) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content:         fmt.Sprintf("Выбор критериев для шаблона '%s'", response.QueueTemplate.Name),
		Components:      components.ChooseCriteriaSelectMenu("queue-template", response.QueueTemplate.ID, response.Criteria, response.SelectedCriteria),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
		Flags:           discordgo.MessageFlagsEphemeral,
	}
}

func queueCriteriaSelectMenuResponse(response *core.SelectCriteriaForQueueResponse) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content:         fmt.Sprintf("Выбор критериев для очереди '%s'", response.Queue.Name),
		Components:      components.ChooseCriteriaSelectMenu("queue", response.Queue.ID, response.Criteria, response.SelectedCriteria),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
		Flags:           discordgo.MessageFlagsEphemeral,
	}
}
