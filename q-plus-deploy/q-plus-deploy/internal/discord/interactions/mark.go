package interactions

import (
	"github.com/bwmarrin/discordgo"
	"q+/internal/core"
	"q+/internal/discord/interactions/handlers"
)

var markCommands = []builder{
	commandF(core.ChannelQueue, handlers.SetMark, &discordgo.ApplicationCommand{
		Name:        "set-mark",
		Description: "Поставить оценку текущим принимаемым студентам",
	},
		autocompleteOption(handlers.AutocompleteCurrentQueuePlaceCriteriaList, &discordgo.ApplicationCommandOption{
			Name:        "criterion",
			Description: "Критерий",
			Type:        discordgo.ApplicationCommandOptionInteger,
			Required:    true,
		}),
		option(&discordgo.ApplicationCommandOption{
			Name:        "mark",
			Description: "Оценка",
			Type:        discordgo.ApplicationCommandOptionString,
			Required:    true,
		}),
	),
}
