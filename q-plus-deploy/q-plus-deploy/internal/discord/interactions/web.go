package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var webCommands = []builder{
	commandF(core.ChannelTeacher, handlers.WebQueueTemplate, &discordgo.ApplicationCommand{
		Name:        "web-queue-template",
		Description: "Веб-интерфейс для создания/копирования шаблона",
	}),
	commandF(core.ChannelTeacher, handlers.WebScheduleQueues, &discordgo.ApplicationCommand{
		Name:        "web-schedule-queues",
		Description: "Веб-интерфейс для планирования очередей",
	},
		autocompleteOption(handlers.AutocompleteQueueTemplateList, &discordgo.ApplicationCommandOption{
			Name:        "template",
			Description: "Шаблон очереди",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
	),
}
