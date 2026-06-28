package channel

import (
	"context"

	"SuperBotGo/internal/model"
)

type ChannelAdapter interface {
	Type() model.ChannelType
	SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error
	SendToChat(ctx context.Context, chatID string, msg model.Message) error
}

// StatusChecker is an optional interface that adapters can implement
// to report their real-time connection status.
type StatusChecker interface {
	Connected() bool
}

// MessageEditor is an optional interface for adapters that support editing
// previously sent messages in place. Pass an empty model.Message to remove
// the inline keyboard without changing the text.
type MessageEditor interface {
	EditMessage(ctx context.Context, chatID string, messageID int, msg model.Message) error
}

// MessageIDSender is an optional interface for adapters that can return the ID
// of the message they just sent. Used to track and later clear stale keyboards.
type MessageIDSender interface {
	SendToChatWithID(ctx context.Context, chatID string, msg model.Message) (int, error)
}

type ChatJoinHandler interface {
	OnChatJoin(ctx context.Context, channelType model.ChannelType, platformChatID string, chatKind model.ChatKind, title string) error
	OnChatLeave(ctx context.Context, channelType model.ChannelType, platformChatID string) error
}
