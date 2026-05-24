package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var userCommands = []builder{
	commandF(core.ChannelAny, handlers.Register, &discordgo.ApplicationCommand{
		Name:        "register",
		Description: "Зарегистрироваться",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "surname",
				Description: "Фамилия",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionString,
			},
			{
				Name:        "name",
				Description: "Имя",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionString,
			},
			{
				Name:        "patronymic",
				Description: "Отчество",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionString,
			},
			{
				Name:        "group",
				Description: "Учебная группа ('0' для преподавателей)",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionString,
			},
			{
				Name:        "gmail",
				Description: "Гугл почта (как в гугл классе)",
				Required:    true,
				Type:        discordgo.ApplicationCommandOptionString,
			},
		},
	}),
}
