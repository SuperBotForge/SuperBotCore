package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var channelsForCourseCommands = []builder{
	command(core.ChannelAny, &discordgo.ApplicationCommand{
		Name:        "channels",
		Description: "Набор каналов для предмета",
	},
		subcommandF(handlers.AddChannelsForCourse, &discordgo.ApplicationCommandOption{
			Name:        "add",
			Description: "Добавить набор каналов для предмета",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название набора (например название предмета)",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:         "queue_channel",
					Description:  "Канал для очереди (команды /next, /set-mark и т.д.)",
					Type:         discordgo.ApplicationCommandOptionChannel,
					ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
					Required:     true,
				},
				{
					Name:         "student_channel",
					Description:  "Канал для студентов (запись студентов в очередь)",
					Type:         discordgo.ApplicationCommandOptionChannel,
					ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
					Required:     true,
				},
				{
					Name:         "teacher_channel",
					Description:  "Канал для преподавателей (настройка бота и запись преподавателей в очередь)",
					Type:         discordgo.ApplicationCommandOptionChannel,
					ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
					Required:     true,
				},
			},
		}),
		subcommandF(handlers.CreateChannelsForCourse, &discordgo.ApplicationCommandOption{
			Name:        "create",
			Description: "Создать набор каналов для предмета",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название набора (например название предмета)",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		}),
	),
}
