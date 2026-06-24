package telegram

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

var (
	_ channel.SilentSender  = (*Adapter)(nil)
	_ channel.StatusChecker = (*Adapter)(nil)
	_ channel.MessageEditor = (*Adapter)(nil)
)

type Adapter struct {
	bot       *tele.Bot
	renderer  *Renderer
	connected *atomic.Bool
	fileStore filestore.FileStore
}

func NewAdapter(bot *tele.Bot, connected *atomic.Bool, fs filestore.FileStore) *Adapter {
	return &Adapter{
		bot:       bot,
		renderer:  NewRenderer(),
		connected: connected,
		fileStore: fs,
	}
}

func (a *Adapter) Connected() bool {
	return a.connected.Load()
}

func (a *Adapter) Type() model.ChannelType {
	return model.ChannelTelegram
}

func (a *Adapter) SendToUser(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message) error {
	return a.sendMessage(ctx, string(platformUserID), msg, false)
}

func (a *Adapter) SendToChat(ctx context.Context, chatID string, msg model.Message) error {
	return a.sendMessage(ctx, chatID, msg, false)
}

func (a *Adapter) SendToUserSilent(ctx context.Context, platformUserID model.PlatformUserID, msg model.Message, silent bool) error {
	return a.sendMessage(ctx, string(platformUserID), msg, silent)
}

func (a *Adapter) SendToChatSilent(ctx context.Context, chatID string, msg model.Message, silent bool) error {
	return a.sendMessage(ctx, chatID, msg, silent)
}

const telegramCaptionMaxLength = 1024

func (a *Adapter) sendMessage(ctx context.Context, chatID string, msg model.Message, silent bool) error {
	if msg.IsEmpty() {
		return fmt.Errorf("telegram: refusing to send empty message to chat %s", chatID)
	}

	rendered := a.renderer.Render(msg)

	if rendered.Text == "" && len(rendered.PhotoURLs) == 0 && len(rendered.FileRefs) == 0 && len(rendered.Keyboard) == 0 {
		return nil // nothing to send after rendering
	}

	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat ID %q: %w", chatID, err)
	}

	recipient := &telegramChat{id: id}

	// Collect photos into an album and non-photo files separately.
	var album tele.Album
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	type docEntry struct {
		name     string
		mimeType string
		reader   io.ReadCloser
	}
	var docs []docEntry

	for _, photoURL := range rendered.PhotoURLs {
		album = append(album, &tele.Photo{File: tele.FromURL(photoURL)})
	}

	if a.fileStore != nil {
		for _, ref := range rendered.FileRefs {
			opened, fErr := channel.OpenFileRef(ctx, a.fileStore, ref)
			if fErr != nil {
				return fmt.Errorf("telegram: get file %q: %w", ref.ID, fErr)
			}
			closers = append(closers, opened.Reader)

			if opened.Ref.FileType == model.FileTypePhoto {
				album = append(album, &tele.Photo{File: tele.FromReader(opened.Reader)})
			} else {
				docs = append(docs, docEntry{name: opened.Ref.Name, mimeType: opened.Ref.MIMEType, reader: opened.Reader})
			}
		}
	}

	// If there are photos, no keyboard, and text fits in a caption —
	// embed text as the album caption instead of sending separately.
	hasKeyboard := len(rendered.Keyboard) > 0
	textAsCaption := len(album) > 0 && !hasKeyboard &&
		rendered.Text != "" && len([]rune(rendered.Text)) <= telegramCaptionMaxLength
	if textAsCaption {
		if p, ok := album[0].(*tele.Photo); ok {
			p.Caption = rendered.Text
		}
	}

	// Send text as a separate message when it wasn't used as caption.
	if rendered.Text != "" && !textAsCaption {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}

		if hasKeyboard {
			opts.ReplyMarkup = buildInlineMarkup(rendered.Keyboard)
		}

		if _, err := a.bot.Send(recipient, rendered.Text, opts); err != nil {
			return fmt.Errorf("telegram: send text: %w", err)
		}
	}

	// Send photos: single photo via Send, multiple via SendAlbum.
	if len(album) == 1 {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}
		if _, err := a.bot.Send(recipient, album[0], opts); err != nil {
			return fmt.Errorf("telegram: send photo: %w", err)
		}
	} else if len(album) > 1 {
		opts := &tele.SendOptions{
			ParseMode:           tele.ModeHTML,
			DisableNotification: silent,
		}
		if _, err := a.bot.SendAlbum(recipient, album, opts); err != nil {
			return fmt.Errorf("telegram: send album: %w", err)
		}
	}

	// Send non-photo files individually.
	for _, d := range docs {
		doc := &tele.Document{
			File:     tele.FromReader(d.reader),
			FileName: d.name,
			MIME:     d.mimeType,
		}
		opts := &tele.SendOptions{
			DisableNotification: silent,
		}
		if _, err := a.bot.Send(recipient, doc, opts); err != nil {
			return fmt.Errorf("telegram: send file: %w", err)
		}
	}

	return nil
}

type telegramChat struct {
	id int64
}

func (c *telegramChat) Recipient() string {
	return strconv.FormatInt(c.id, 10)
}

// editableRef implements tele.Editable so we can edit a message by chatID + messageID.
type editableRef struct {
	msgID  int
	chatID int64
}

func (e editableRef) MessageSig() (string, int64) {
	return strconv.Itoa(e.msgID), e.chatID
}

// EditMessage edits a previously sent message in place.
// Pass an empty msg to remove the inline keyboard without changing text.
func (a *Adapter) EditMessage(ctx context.Context, chatID string, messageID int, msg model.Message) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram edit: invalid chat ID %q: %w", chatID, err)
	}

	editable := editableRef{msgID: messageID, chatID: id}

	if msg.IsEmpty() {
		// Remove inline keyboard, keep existing text
		_, err = a.bot.Edit(editable, &tele.ReplyMarkup{})
		return ignoreNotModified(err)
	}

	rendered := a.renderer.Render(msg)

	// Can't edit messages that require sending new media
	if len(rendered.PhotoURLs) > 0 || len(rendered.FileRefs) > 0 {
		return nil
	}

	if rendered.Text == "" {
		return nil
	}

	opts := &tele.SendOptions{ParseMode: tele.ModeHTML}
	opts.ReplyMarkup = buildInlineMarkup(rendered.Keyboard)

	_, err = a.bot.Edit(editable, rendered.Text, opts)
	return ignoreNotModified(err)
}

func buildInlineMarkup(keyboard [][]InlineButton) *tele.ReplyMarkup {
	markup := &tele.ReplyMarkup{}
	if len(keyboard) == 0 {
		return markup // empty markup removes the keyboard
	}
	rows := make([]tele.Row, 0, len(keyboard))
	for _, kbRow := range keyboard {
		btns := make([]tele.Btn, 0, len(kbRow))
		for _, btn := range kbRow {
			btns = append(btns, markup.Data(btn.Text, btn.CallbackData, btn.CallbackData))
		}
		rows = append(rows, markup.Row(btns...))
	}
	markup.Inline(rows...)
	return markup
}

func ignoreNotModified(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}
