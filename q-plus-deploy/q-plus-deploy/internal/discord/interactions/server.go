package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
	"q+/internal/generated/ent/discordrole"
)

var serverCommands = []builder{
	commandF(core.ChannelAny, handlers.Setup, &discordgo.ApplicationCommand{
		Name:        "setup",
		Description: "Настроить права на сервере (нужно вызвать один раз после настройки каналов для бота на сервере)",
	}),
	command(core.ChannelAny, &discordgo.ApplicationCommand{
		Name:        "role",
		Description: "Настройка ролей студентов и принимающих у бота на сервере",
	},
		subcommandF(handlers.AddRole, &discordgo.ApplicationCommandOption{
			Name:        "add",
			Description: "Добавить роль в бота на сервер",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Тип роли (студент или принимающий)",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Студент",
							Value: discordrole.TypeStudent,
						},
						{
							Name:  "Принимающий",
							Value: discordrole.TypeExaminer,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "role",
					Description: "Discord роль",
					Required:    true,
				},
			},
		}),
		subcommandF(handlers.RemoveRole, &discordgo.ApplicationCommandOption{
			Name:        "remove",
			Description: "Удалить роль из бота",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        "role",
					Description: "Discord роль",
					Required:    true,
				},
			},
		}),
		subcommandF(handlers.ListRoles, &discordgo.ApplicationCommandOption{
			Name:        "list",
			Description: "Список запомненных ролей",
		}),
	),
}
