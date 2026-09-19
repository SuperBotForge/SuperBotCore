package vk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
)

var _ channel.StatusChecker = (*Adapter)(nil)

type Adapter struct {
	vk        *vkapi.VK
	renderer  *Renderer
	connected *atomic.Bool
	fileStore filestore.FileStore
}

func NewAdapter(vk *vkapi.VK, connected *atomic.Bool, fs filestore.FileStore) *Adapter {
	return &Adapter{
		vk:        vk,
		renderer:  NewRenderer(),
		connected: connected,
		fileStore: fs,
	}
}

func (a *Adapter) Connected() bool {
	return a.connected.Load()
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelVK
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return a.sendMessage(ctx, string(platformUserID), msg)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg)
}

func (a *Adapter) sendMessage(ctx context.Context, chatID string, msg model.Message) error {
	_, err := a.SendToChatWithID(ctx, chatID, msg)
	return err
}

func (a *Adapter) SendToChatWithID(ctx context.Context, chatID string, msg model.Message) (string, error) {
	if msg.IsEmpty() {
		return "", fmt.Errorf("vk: refusing to send empty message to peer %s", chatID)
	}

	rendered := a.renderer.Render(msg)

	peerID, err := strconv.Atoi(chatID)
	if err != nil {
		return "", fmt.Errorf("vk: invalid peer ID %q: %w", chatID, err)
	}

	text := appendURLLines(rendered.Text, rendered.ImageURLs)
	attachments, err := a.uploadFiles(ctx, peerID, rendered.FileRefs)
	if err != nil {
		return "", err
	}

	if text == "" && len(attachments) == 0 {
		return "", nil
	}

	params := vkapi.Params{
		"peer_id":   peerID,
		"random_id": rand.IntN(1_000_000_000),
	}
	if text != "" {
		params["message"] = text
	}
	if rendered.FormatData != nil {
		params["format_data"] = rendered.FormatData
	}
	if len(attachments) > 0 {
		params["attachment"] = strings.Join(attachments, ",")
	}
	if rendered.Keyboard != nil {
		params["keyboard"] = rendered.Keyboard
	}

	id, err := a.vk.MessagesSend(params.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("vk: send message to %s: %w", chatID, err)
	}
	return strconv.Itoa(id), nil
}

func (a *Adapter) uploadFiles(ctx context.Context, peerID int, refs []model.FileRef) ([]string, error) {
	if a.fileStore == nil || len(refs) == 0 {
		return nil, nil
	}

	attachments := make([]string, 0, len(refs))
	for _, ref := range refs {
		opened, err := channel.OpenFileRef(ctx, a.fileStore, ref)
		if err != nil {
			return nil, fmt.Errorf("vk: get file %q: %w", ref.ID, err)
		}

		name := opened.Ref.Name
		mimeType := opened.Ref.MIMEType
		fileType := opened.Ref.FileType
		if name == "" {
			name = ref.ID
		}

		var attachment string
		switch {
		case fileType == model.FileTypePhoto || strings.HasPrefix(mimeType, "image/"):
			photos, upErr := a.vk.UploadMessagesPhoto(peerID, opened.Reader)
			_ = opened.Reader.Close()
			if upErr != nil {
				return nil, fmt.Errorf("vk: upload photo %q: %w", name, upErr)
			}
			if len(photos) == 0 {
				return nil, fmt.Errorf("vk: upload photo %q returned no attachments", name)
			}
			attachment = photos[0].ToAttachment()
		default:
			// Reopen for every attempt: a failed upload consumes the reader.
			_ = opened.Reader.Close()
			doc, upErr := a.uploadDocument(ctx, peerID, ref, fileType, name)
			if upErr != nil {
				return nil, fmt.Errorf("vk: upload document %q: %w", name, upErr)
			}
			switch {
			case doc.Doc.ID != 0:
				attachment = doc.Doc.ToAttachment()
			case doc.AudioMessage.ID != 0:
				attachment = doc.AudioMessage.ToAttachment()
			case doc.Graffiti.ID != 0:
				attachment = doc.Graffiti.ToAttachment()
			default:
				return nil, fmt.Errorf("vk: upload document %q returned no attachment", name)
			}
		}

		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// Only retry the known transient upload-server storage error. Each SDK call
// obtains a fresh upload URL; messages.send is never retried here.
func (a *Adapter) uploadDocument(ctx context.Context, peerID int, ref model.FileRef, fileType model.FileType, name string) (vkapi.DocsSaveResponse, error) {
	var result vkapi.DocsSaveResponse
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if attempt > 0 {
			if err = sleepWithContext(ctx, time.Duration(attempt)*500*time.Millisecond); err != nil {
				return result, err
			}
		}
		opened, openErr := channel.OpenFileRef(ctx, a.fileStore, ref)
		if openErr != nil {
			return result, openErr
		}
		result, err = a.vk.UploadMessagesDoc(peerID, docTypeFor(fileType), name, "", opened.Reader)
		_ = opened.Reader.Close()
		var uploadErr *vkapi.UploadError
		if !errors.As(err, &uploadErr) || !strings.HasPrefix(uploadErr.Err, "no_free_space") {
			return result, err
		}
	}
	return result, err
}

func docTypeFor(fileType model.FileType) string {
	switch fileType {
	case model.FileTypeVoice:
		return "audio_message"
	case model.FileTypeSticker:
		return "graffiti"
	default:
		return "doc"
	}
}

func appendURLLines(text string, urls []string) string {
	if len(urls) == 0 {
		return text
	}

	parts := make([]string, 0, len(urls)+1)
	if text != "" {
		parts = append(parts, text)
	}
	parts = append(parts, urls...)
	return strings.Join(parts, "\n")
}

func fallbackFileName(rawURL, current string) string {
	if current != "" {
		return current
	}
	if base := path.Base(rawURL); base != "" && base != "." && base != "/" {
		return base
	}
	return fmt.Sprintf("file-%d", time.Now().UnixNano())
}

func inferFileType(name, mimeType string, fallback model.FileType) model.FileType {
	if fallback != "" {
		return fallback
	}
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return model.FileTypePhoto
	case strings.HasPrefix(mimeType, "audio/"):
		return model.FileTypeAudio
	case strings.HasPrefix(mimeType, "video/"):
		return model.FileTypeVideo
	}

	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ogg"):
		return model.FileTypeVoice
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"), strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".gif"), strings.HasSuffix(lower, ".webp"):
		return model.FileTypePhoto
	case strings.HasSuffix(lower, ".mp3"), strings.HasSuffix(lower, ".wav"), strings.HasSuffix(lower, ".m4a"):
		return model.FileTypeAudio
	case strings.HasSuffix(lower, ".mp4"), strings.HasSuffix(lower, ".mov"), strings.HasSuffix(lower, ".avi"), strings.HasSuffix(lower, ".webm"):
		return model.FileTypeVideo
	default:
		return model.FileTypeDocument
	}
}

func storeDownloadedFile(ctx context.Context, fs filestore.FileStore, meta filestore.FileMeta, body io.Reader) (model.FileRef, error) {
	return fs.Store(ctx, meta, body)
}
