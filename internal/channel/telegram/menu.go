package telegram

import (
	"context"
	"strconv"
)

func (a *Adapter) DeleteMessage(_ context.Context, chatID, messageID string) error {
	chat, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(messageID)
	if err != nil {
		return err
	}
	return a.bot.Delete(editableRef{msgID: id, chatID: chat})
}
