package mattermost

import "context"

func (a *Adapter) DeleteMessage(ctx context.Context, chatID, messageID string) error {
	_, err := a.client.DeletePost(ctx, messageID)
	return err
}
