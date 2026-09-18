package vk

import (
	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
	"context"
	"fmt"
	vkapi "github.com/SevereCloud/vksdk/v3/api"
	"strconv"
)

func (a *Adapter) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	peer, err := strconv.Atoi(chatID)
	if err != nil {
		return err
	}
	result, err := a.vk.MessagesDelete(vkapi.Params{"peer_id": peer, "message_ids": messageID, "delete_for_all": 1}.WithContext(ctx))
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return fmt.Errorf("vk: empty delete result for %s", messageID)
	}
	for _, item := range result {
		if item.Error != nil || item.Response == nil || !bool(*item.Response) {
			return fmt.Errorf("vk: deleting menu %s failed: %+v", messageID, item)
		}
	}
	return nil
}

func (a *Adapter) EditMessage(ctx context.Context, chatID, messageID string, msg model.Message) error {
	peer, err := strconv.Atoi(chatID)
	if err != nil {
		return err
	}
	params := vkapi.Params{"peer_id": peer, "message_id": messageID, "keyboard": `{"inline":true,"buttons":[]}`}
	if !msg.IsEmpty() {
		rendered := a.renderer.Render(msg)
		if len(rendered.FileRefs) > 0 || len(rendered.ImageURLs) > 0 {
			return channel.ErrMessageOperationUnsupported
		}
		params["message"] = rendered.Text
		if rendered.Keyboard != nil {
			params["keyboard"] = rendered.Keyboard
		}
		if rendered.FormatData != nil {
			params["format_data"] = rendered.FormatData
		}
	}
	_, err = a.vk.MessagesEdit(params.WithContext(ctx))
	return err
}
