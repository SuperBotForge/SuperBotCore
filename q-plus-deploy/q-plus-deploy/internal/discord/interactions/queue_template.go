package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var queueTemplateCommands = []builder{
	command(core.ChannelTeacher, &discordgo.ApplicationCommand{
		Name:        "queue-template",
		Description: "Шаблон очереди",
	},
		subcommandF(handlers.QueueTemplateCreate, &discordgo.ApplicationCommandOption{
			Name:        "create",
			Description: "Создать шаблон очереди",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "sign_up_lead_time",
					Description: "За сколько времени до начала очереди открывать запись (в формате 1d2h30m)",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		}),
		subcommandF(handlers.QueueTemplateList, &discordgo.ApplicationCommandOption{
			Name:        "list",
			Description: "Список всех шаблонов очередей",
		}),
		subcommandF(handlers.QueueTemplateEdit, &discordgo.ApplicationCommandOption{
			Name:        "edit",
			Description: "Отредактировать шаблон очереди",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название",
					Type:        discordgo.ApplicationCommandOptionString,
				},
				{
					Name:        "sign_up_lead_time",
					Description: "За сколько времени до начала очереди открывать запись (в формате 1d2h30m)",
					Type:        discordgo.ApplicationCommandOptionString,
				},
			},
		},
			autocompleteOption(handlers.AutocompleteQueueTemplateList, &discordgo.ApplicationCommandOption{
				Name:        "template",
				Description: "Шаблон очереди",
				Type:        discordgo.ApplicationCommandOptionInteger,
				Required:    true,
			}),
		),
		subcommandGroup(&discordgo.ApplicationCommandOption{
			Name:        "criterion",
			Description: "Критерии",
		},
			subcommandF(handlers.QueueTemplateSelectCriteria, &discordgo.ApplicationCommandOption{
				Name:        "select",
				Description: "Выбрать критерии для шаблона из списка критериев предмета",
			},
				autocompleteOption(handlers.AutocompleteQueueTemplateList, &discordgo.ApplicationCommandOption{
					Name:        "template",
					Description: "Шаблон очереди",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				}),
			),
		),
	),
}
