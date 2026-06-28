package mattermost

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	mm "github.com/mattermost/mattermost/server/public/model"
)

var (
	_ channel.StatusChecker   = (*Adapter)(nil)
	_ channel.MessageIDSender = (*Adapter)(nil)
	_ channel.MessageEditor   = (*Adapter)(nil)
)

type Adapter struct {
	client    *mm.Client4
	renderer  *Renderer
	connected *atomic.Bool
	fileStore filestore.FileStore
	botUserID string
	actions   ActionConfig
}

type ActionConfig struct {
	URL    string
	Secret string
}

func NewAdapter(client *mm.Client4, botUserID string, connected *atomic.Bool, fs filestore.FileStore, actions ActionConfig) *Adapter {
	return &Adapter{
		client:    client,
		renderer:  NewRenderer(),
		connected: connected,
		fileStore: fs,
		botUserID: botUserID,
		actions:   actions,
	}
}

func (a *Adapter) Connected() bool {
	return a.connected.Load()
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelMattermost
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	if a.botUserID == "" {
		return fmt.Errorf("mattermost: bot user id is not initialized")
	}

	dm, _, err := a.client.CreateDirectChannel(ctx, a.botUserID, string(platformUserID))
	if err != nil {
		return fmt.Errorf("mattermost: create DM for user %s: %w", platformUserID, err)
	}
	return a.sendMessage(ctx, dm.Id, msg)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	_, err := a.sendMessageGetID(ctx, chatID, msg)
	return err
}

func (a *Adapter) SendToChatWithID(ctx context.Context, chatID string, msg model.Message) (string, error) {
	return a.sendMessageGetID(ctx, chatID, msg)
}

func (a *Adapter) EditMessage(ctx context.Context, chatID string, messageID string, msg model.Message) error {
	patch := &mm.PostPatch{}
	if msg.IsEmpty() {
		// Remove interactive attachments by setting empty props.
		emptyAttachments := []*mm.SlackAttachment{}
		props := mm.StringInterface{mm.PostPropsAttachments: emptyAttachments}
		patch.Props = &props
	} else {
		rendered := a.renderer.Render(msg)
		text := appendURLLines(rendered.Text, rendered.ImageURLs)
		patch.Message = &text
		if rendered.Options != nil && len(rendered.Options.Options) > 0 {
			attachments := a.buildActionAttachments(rendered.Options)
			props := mm.StringInterface{mm.PostPropsAttachments: attachments}
			patch.Props = &props
		}
	}
	if _, _, err := a.client.PatchPost(ctx, messageID, patch); err != nil {
		return fmt.Errorf("mattermost: patch post %s: %w", messageID, err)
	}
	return nil
}

func (a *Adapter) sendMessage(ctx context.Context, chatID string, msg model.Message) error {
	_, err := a.sendMessageGetID(ctx, chatID, msg)
	return err
}

func (a *Adapter) sendMessageGetID(ctx context.Context, chatID string, msg model.Message) (string, error) {
	if msg.IsEmpty() {
		return "", fmt.Errorf("mattermost: refusing to send empty message to channel %s", chatID)
	}

	rendered := a.renderer.Render(msg)
	postText := appendURLLines(rendered.Text, rendered.ImageURLs)
	if rendered.Options != nil && len(rendered.Options.Options) > 0 && !a.hasInteractiveActions() {
		postText = appendLines(postText, renderOptions(rendered.Options.Options))
	}
	fileIDs, err := a.uploadFiles(ctx, chatID, rendered.FileRefs)
	if err != nil {
		return "", err
	}
	attachments := a.buildActionAttachments(rendered.Options)

	if postText == "" && len(fileIDs) == 0 && len(attachments) == 0 {
		return "", nil
	}

	post := &mm.Post{
		ChannelId: chatID,
		Message:   postText,
		FileIds:   fileIDs,
	}
	if len(attachments) > 0 {
		post.AddProp(mm.PostPropsAttachments, attachments)
	}
	created, _, err := a.client.CreatePost(ctx, post)
	if err != nil {
		return "", fmt.Errorf("mattermost: create post in %s: %w", chatID, err)
	}
	return created.Id, nil
}

func (a *Adapter) hasInteractiveActions() bool {
	return a.actions.URL != "" && a.actions.Secret != ""
}

func (a *Adapter) buildActionAttachments(options *model.OptionsBlock) []*mm.MessageAttachment {
	if options == nil || len(options.Options) == 0 || !a.hasInteractiveActions() {
		return nil
	}

	actions := make([]*mm.PostAction, 0, len(options.Options))
	for _, opt := range options.Options {
		if opt.Value == "" {
			continue
		}

		label := channel.OptionLabel(opt)
		if label == "" {
			continue
		}

		actions = append(actions, &mm.PostAction{
			Type: mm.PostActionTypeButton,
			Name: label,
			Integration: &mm.PostActionIntegration{
				URL: a.actions.URL,
				Context: map[string]any{
					actionContextValueKey:  opt.Value,
					actionContextLabelKey:  label,
					actionContextSecretKey: a.actions.Secret,
				},
			},
		})
	}

	if len(actions) == 0 {
		return nil
	}

	return []*mm.MessageAttachment{{
		Fallback: "Select an option",
		Actions:  actions,
	}}
}

func (a *Adapter) uploadFiles(ctx context.Context, channelID string, refs []model.FileRef) ([]string, error) {
	if a.fileStore == nil || len(refs) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		opened, err := channel.OpenFileRef(ctx, a.fileStore, ref)
		if err != nil {
			return nil, fmt.Errorf("mattermost: get file %q: %w", ref.ID, err)
		}

		data, err := io.ReadAll(opened.Reader)
		_ = opened.Reader.Close()
		if err != nil {
			return nil, fmt.Errorf("mattermost: read file %q: %w", ref.ID, err)
		}

		name := opened.Ref.Name
		if name == "" {
			name = fmt.Sprintf("file-%d", time.Now().UnixNano())
		}

		upload, _, err := a.client.UploadFile(ctx, data, channelID, name)
		if err != nil {
			return nil, fmt.Errorf("mattermost: upload file %q: %w", name, err)
		}
		ids = append(ids, upload.FileInfos[0].Id)
	}

	return ids, nil
}

func appendURLLines(text string, urls []string) string {
	if len(urls) == 0 {
		return text
	}

	var builder strings.Builder
	if text != "" {
		builder.WriteString(text)
	}
	for _, url := range urls {
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(url)
	}
	return builder.String()
}

func appendLines(text string, lines []string) string {
	if len(lines) == 0 {
		return text
	}

	var builder strings.Builder
	if text != "" {
		builder.WriteString(text)
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
	}
	return builder.String()
}

func fileMetaFromMattermost(info *mm.FileInfo) filestore.FileMeta {
	return filestore.FileMeta{
		Name:     info.Name,
		MIMEType: info.MimeType,
		Size:     info.Size,
		FileType: detectMattermostFileType(info.Name, info.MimeType),
	}
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
