package interactions

import (
	"errors"
	"github.com/bwmarrin/discordgo"
	"q+/internal/discord/interactions/components"
	"q+/internal/discord/interactions/handlers"
	"strconv"
	"strings"
)

var allComponents = []componentWithHandler{
	button(components.CriteriaEditButton, handlers.EditCriteriaModal),
	modal(components.CriteriaEditModal, handlers.EditCriteria),
	selectMenu(components.CriteriaSelectMenu, handlers.ChooseCriteria),
}

type componentWithHandler interface {
	customId() string
	handler() handlers.ComponentHandler
}

type buttonWithHandler struct {
	discordgo.Button
	h handlers.ComponentHandler
}

func (b buttonWithHandler) customId() string {
	return b.CustomID
}

func (b buttonWithHandler) handler() handlers.ComponentHandler {
	return b.h
}

func button(button discordgo.Button, handler handlers.ComponentHandler) buttonWithHandler {
	return buttonWithHandler{
		Button: button,
		h:      handler,
	}
}

type selectMenuWithHandler struct {
	discordgo.SelectMenu
	h handlers.ComponentHandler
}

func (s selectMenuWithHandler) customId() string {
	return s.CustomID
}

func (s selectMenuWithHandler) handler() handlers.ComponentHandler {
	return s.h
}

func selectMenu(menu discordgo.SelectMenu, handler handlers.ComponentHandler) selectMenuWithHandler {
	return selectMenuWithHandler{
		SelectMenu: menu,
		h:          handler,
	}
}

type modalWithHandler struct {
	discordgo.InteractionResponseData
	h handlers.ComponentHandler
}

func (m modalWithHandler) customId() string {
	return m.CustomID
}

func (m modalWithHandler) handler() handlers.ComponentHandler {
	return m.h
}

func modal(modal discordgo.InteractionResponseData, handler handlers.ComponentHandler) modalWithHandler {
	return modalWithHandler{
		InteractionResponseData: modal,
		h:                       handler,
	}
}

func BuildComponentsHandler() handlers.EntryHandler {
	handlersMap := make(map[string]handlers.ComponentHandler)
	for _, component := range allComponents {
		handlersMap[component.customId()] = component.handler()
	}
	return func(ctx handlers.InteractionContext) error {
		var customIdData []string
		if ctx.I.Type == discordgo.InteractionMessageComponent {
			customIdData = strings.Split(ctx.I.MessageComponentData().CustomID, "_")
		} else if ctx.I.Type == discordgo.InteractionModalSubmit {
			customIdData = strings.Split(ctx.I.ModalSubmitData().CustomID, "_")
		} else {
			return errors.New("unknown interaction type")
		}
		customId := customIdData[0]
		entity := customIdData[1]
		entityId, err := strconv.ParseInt(customIdData[2], 10, 64)
		if err != nil {
			return err
		}
		if handler, ok := handlersMap[customId]; ok {
			return handler(ctx, entity, entityId)
		} else {
			return errors.New("unknown component")
		}
	}
}
