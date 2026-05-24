package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var courseInstanceCommands = []builder{
	command(core.ChannelTeacher, &discordgo.ApplicationCommand{
		Name:        "course",
		Description: "Предмет",
	},
		subcommandF(handlers.CourseInstanceCreate, &discordgo.ApplicationCommandOption{
			Name:        "create",
			Description: "Создать предмет",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Название предмета",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		}),
		subcommandGroup(&discordgo.ApplicationCommandOption{
			Name:        "criterion",
			Description: "Критерий",
		},
			subcommandF(handlers.CourseCriterionWizard, &discordgo.ApplicationCommandOption{
				Name:        "wizard",
				Description: "Редактирование всех критериев предмета",
			}),
		),
	),
}
