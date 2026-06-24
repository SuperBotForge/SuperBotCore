package telegram

import (
	"context"
	"strings"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/model"
)

func CallbackNormalizer() channel.UpdateMiddleware {
	return func(next channel.UpdateHandlerFunc) channel.UpdateHandlerFunc {
		return func(ctx context.Context, u channel.Update) error {
			if cb, ok := u.Input.(model.CallbackInput); ok {
				u.Input = model.CallbackInput{
					Data:      stripTelebotPrefix(cb.Data),
					Label:     cb.Label,
					MessageID: cb.MessageID,
				}
			}
			return next(ctx, u)
		}
	}
}

func stripTelebotPrefix(data string) string {
	if len(data) == 0 || data[0] != '\f' {
		return data
	}
	rest := data[1:]
	if idx := strings.IndexByte(rest, '|'); idx >= 0 {
		return rest[idx+1:]
	}
	if idx := strings.IndexByte(rest, '\f'); idx >= 0 {
		return rest[idx+1:]
	}
	return rest
}
