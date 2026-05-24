package components

import (
	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"q+/internal/generated/ent"
	"q+/internal/utils"
	"strconv"
)

func CriteriaWizardButtons(entity string, entityId int64) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				Button(CriteriaEditButton, entity, entityId),
			},
		},
	}
}

var CriteriaEditButton = discordgo.Button{
	CustomID: "btn-criteria-edit",
	Label:    "Редактировать",
	Style:    discordgo.PrimaryButton,
	Disabled: false,
	Emoji: &discordgo.ComponentEmoji{
		Name: "📝",
	},
}

func FillCriteriaEditModal(entity string, entityId int64, title string, body string) *discordgo.InteractionResponseData {
	modal := Modal(CriteriaEditModal, entity, entityId)
	modal.Title = title
	modal.Components = []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    "criteria",
					Label:       "критерии с тем же номером будут изменены",
					Style:       discordgo.TextInputParagraph,
					Placeholder: "1. критерий1\n2. критерий2",
					Value:       body,
					Required:    false,
					MinLength:   0,
					MaxLength:   4000,
				},
			},
		},
		// TODO instruction to use criteria edit modal
		//discordgo.ActionsRow{
		//	Components: []discordgo.MessageComponent{
		//		discordgo.TextInput{
		//			CustomID:    "criteria2",
		//			Label:       "критерии с тем же номером будут изменены",
		//			Style:       discordgo.TextInputParagraph,
		//			Placeholder: "1. критерий1\n2. критерий2",
		//			Value:       "",
		//			Required:    false,
		//			MaxLength:   4000,
		//		},
		//	},
		//},
	}
	return &modal
}

var CriteriaEditModal = discordgo.InteractionResponseData{
	CustomID: "modal-criteria-edit",
}

func ChooseCriteriaSelectMenu(entity string, entityId int64, criteria []*ent.Criterion, selectedCriteria []int64) []discordgo.MessageComponent {
	selectedSet := make(map[int64]struct{}, len(selectedCriteria))
	for _, id := range selectedCriteria {
		selectedSet[id] = struct{}{}
	}
	options := lo.Map(criteria, func(criterion *ent.Criterion, _ int) discordgo.SelectMenuOption {
		_, selected := selectedSet[criterion.ID]
		return discordgo.SelectMenuOption{
			Label:   utils.LimitString(criterion.Name, 100),
			Value:   strconv.FormatInt(criterion.ID, 10),
			Default: selected,
		}
	})

	selectMenu := SelectMenu(CriteriaSelectMenu, entity, entityId)
	selectMenu.MaxValues = len(options)
	selectMenu.Options = options
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				selectMenu,
			},
		},
	}
}

var CriteriaSelectMenu = discordgo.SelectMenu{
	CustomID:    "choose-criteria",
	Placeholder: "Выберите критерии",
	MinValues:   lo.ToPtr(0),
}
