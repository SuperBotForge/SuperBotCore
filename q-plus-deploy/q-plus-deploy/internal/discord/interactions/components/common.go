package components

import (
	"github.com/bwmarrin/discordgo"
	"strconv"
)

func Button(btn discordgo.Button, entity string, entityId int64) discordgo.Button {
	btn.CustomID = btn.CustomID + "_" + entity + "_" + strconv.FormatInt(entityId, 10)
	return btn
}

func SelectMenu(menu discordgo.SelectMenu, entity string, entityId int64) discordgo.SelectMenu {
	menu.CustomID = menu.CustomID + "_" + entity + "_" + strconv.FormatInt(entityId, 10)
	return menu

}

func Modal(modal discordgo.InteractionResponseData, entity string, entityId int64) discordgo.InteractionResponseData {
	modal.CustomID = modal.CustomID + "_" + entity + "_" + strconv.FormatInt(entityId, 10)
	return modal
}
