package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	tele "gopkg.in/telebot.v3"
)

type BotConfig struct {
	Token         string
	Mode          string // "polling" (default) or "webhook"
	WebhookURL    string
	WebhookSecret string
	WebhookListen string
}

type Bot struct {
	bot          *tele.Bot
	handler      channel.UpdateHandlerFunc
	joinHandler  channel.ChatJoinHandler
	fileStore    filestore.FileStore
	maxFileSize  int64
	logger       *slog.Logger
	connected    atomic.Bool
	mode         string
	albums       *albumBuffer
	lifecycleCtx context.Context
}

type telegramIncomingFile struct {
	file     *tele.File
	name     string
	mimeType string
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, fs filestore.FileStore, maxFileSize int64, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "polling"
	}

	var poller tele.Poller
	switch mode {
	case "webhook":
		wh := &tele.Webhook{
			SecretToken: cfg.WebhookSecret,
			Endpoint:    &tele.WebhookEndpoint{PublicURL: cfg.WebhookURL},
		}
		if cfg.WebhookListen != "" {
			wh.Listen = cfg.WebhookListen
		}
		poller = wh
	default:
		poller = &tele.LongPoller{Timeout: 10 * time.Second}
	}

	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: poller,
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("telegram: create bot: %w", err)
	}

	tb := &Bot{
		bot:         b,
		handler:     handler,
		joinHandler: joinHandler,
		fileStore:   fs,
		maxFileSize: maxFileSize,
		logger:      logger,
		mode:        mode,
	}

	tb.albums = newAlbumBuffer(func(album *pendingAlbum) {
		tb.flushAlbum(album)
	})

	tb.registerHandlers()

	return tb, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.bot, &b.connected, b.fileStore)
}

func (b *Bot) Start(ctx context.Context) error {
	b.logger.Info("Telegram bot starting", slog.String("mode", b.mode))
	b.lifecycleCtx = ctx
	b.connected.Store(true)

	go func() {
		<-ctx.Done()
		b.logger.Info("Telegram bot stopping")
		b.connected.Store(false)
		b.bot.Stop()
	}()

	b.bot.Start()
	return nil
}

// deriveContext returns a context derived from the bot lifecycle context.
func (b *Bot) deriveContext() context.Context {
	if b.lifecycleCtx != nil {
		return b.lifecycleCtx
	}
	return context.Background()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) RegisterCommands(commands []string) {
	for _, cmd := range commands {
		name := cmd
		b.bot.Handle("/"+name, func(c tele.Context) error {
			return b.handleTextMessage(c)
		})
	}
}

func (b *Bot) handleTextMessage(c tele.Context) error {
	chatID := strconv.FormatInt(c.Chat().ID, 10)
	platformUserID := strconv.FormatInt(c.Sender().ID, 10)
	updateID := strconv.Itoa(c.Update().ID)
	text := c.Text()

	b.logger.Info("telegram: received message",
		slog.String("user", platformUserID),
		slog.String("chat", chatID),
		slog.String("text", text))

	ctx := b.deriveContext()
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "tg:" + updateID,
		Input:            model.TextInput{Text: text},
		ChatID:           chatID,
		Username:         c.Sender().Username,
	}); err != nil {
		b.logger.Error("telegram: error handling message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
}

func (b *Bot) handleFileMessage(c tele.Context, fileType model.FileType) error {
	if b.fileStore == nil {
		return nil
	}

	msg := c.Message()
	incoming := extractIncomingTelegramFile(msg, fileType)
	if incoming == nil {
		return nil
	}

	chatID := strconv.FormatInt(c.Chat().ID, 10)
	platformUserID := strconv.FormatInt(c.Sender().ID, 10)
	updateID := strconv.Itoa(c.Update().ID)

	if b.isFileTooLarge(platformUserID, incoming.file) {
		return nil
	}

	ref, err := b.storeTelegramFile(b.deriveContext(), platformUserID, fileType, incoming)
	if err != nil {
		b.logger.Error("telegram: failed to persist file",
			slog.String("user", platformUserID),
			slog.Any("error", err))
		return nil
	}

	b.logger.Info("telegram: received file",
		slog.String("user", platformUserID),
		slog.String("chat", chatID),
		slog.String("file_id", ref.ID),
		slog.String("file_type", string(fileType)),
		slog.String("album_id", msg.AlbumID))

	entry := b.buildAlbumEntry(c, ref)
	if b.albums.add(msg.AlbumID, entry) {
		return nil
	}

	if err := b.dispatchFileUpdate(b.deriveContext(), chatID, platformUserID, updateID, c.Sender().Username, entry.caption, []model.FileRef{ref}); err != nil {
		b.logger.Error("telegram: error handling file message",
			slog.String("user", platformUserID),
			slog.Any("error", err))
	}
	return nil
}

func extractIncomingTelegramFile(msg *tele.Message, fileType model.FileType) *telegramIncomingFile {
	switch fileType {
	case model.FileTypePhoto:
		if msg.Photo == nil {
			return nil
		}
		return &telegramIncomingFile{
			file:     &msg.Photo.File,
			name:     "photo.jpg",
			mimeType: "image/jpeg",
		}
	case model.FileTypeDocument:
		if msg.Document == nil {
			return nil
		}
		return &telegramIncomingFile{
			file:     &msg.Document.File,
			name:     msg.Document.FileName,
			mimeType: msg.Document.MIME,
		}
	case model.FileTypeAudio:
		if msg.Audio == nil {
			return nil
		}
		name := msg.Audio.FileName
		if name == "" {
			name = "audio"
		}
		return &telegramIncomingFile{
			file:     &msg.Audio.File,
			name:     name,
			mimeType: msg.Audio.MIME,
		}
	case model.FileTypeVideo:
		if msg.Video == nil {
			return nil
		}
		name := msg.Video.FileName
		if name == "" {
			name = "video.mp4"
		}
		return &telegramIncomingFile{
			file:     &msg.Video.File,
			name:     name,
			mimeType: msg.Video.MIME,
		}
	case model.FileTypeVoice:
		if msg.Voice == nil {
			return nil
		}
		return &telegramIncomingFile{
			file:     &msg.Voice.File,
			name:     "voice.ogg",
			mimeType: msg.Voice.MIME,
		}
	default:
		return nil
	}
}

func (b *Bot) isFileTooLarge(platformUserID string, file *tele.File) bool {
	if b.maxFileSize > 0 && int64(file.FileSize) > b.maxFileSize {
		b.logger.Warn("telegram: file too large, ignoring",
			slog.String("user", platformUserID),
			slog.Int64("file_size", file.FileSize),
			slog.Int64("max_size", b.maxFileSize))
		return true
	}
	return false
}

func (b *Bot) storeTelegramFile(ctx context.Context, platformUserID string, fileType model.FileType, incoming *telegramIncomingFile) (model.FileRef, error) {
	reader, err := b.bot.File(incoming.file)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("download file for user %s: %w", platformUserID, err)
	}
	defer reader.Close()

	ref, err := b.fileStore.Store(ctx, filestore.FileMeta{
		Name:     incoming.name,
		MIMEType: incoming.mimeType,
		Size:     int64(incoming.file.FileSize),
		FileType: fileType,
	}, reader)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("store file for user %s: %w", platformUserID, err)
	}
	return ref, nil
}

func (b *Bot) buildAlbumEntry(c tele.Context, ref model.FileRef) albumEntry {
	caption := c.Message().Caption
	return albumEntry{
		ref:      ref,
		caption:  caption,
		chatID:   strconv.FormatInt(c.Chat().ID, 10),
		userID:   strconv.FormatInt(c.Sender().ID, 10),
		updateID: strconv.Itoa(c.Update().ID),
		username: c.Sender().Username,
	}
}

func (b *Bot) dispatchFileUpdate(ctx context.Context, chatID, platformUserID, updateID, username, caption string, files []model.FileRef) error {
	return b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelTelegram,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "tg:" + updateID,
		Input:            model.FileInput{Caption: caption, Files: files},
		ChatID:           chatID,
		Username:         username,
	})
}

func collectAlbumFiles(entries []albumEntry) ([]model.FileRef, string) {
	refs := make([]model.FileRef, len(entries))
	for i, entry := range entries {
		refs[i] = entry.ref
	}

	for _, entry := range entries {
		if entry.caption != "" {
			return refs, entry.caption
		}
	}
	return refs, ""
}

// flushAlbum is called when an album's flush timer fires. It combines all
// buffered files into a single FileInput and dispatches it.
func (b *Bot) flushAlbum(album *pendingAlbum) {
	if len(album.entries) == 0 {
		return
	}

	first := album.entries[0]
	refs, caption := collectAlbumFiles(album.entries)

	b.logger.Info("telegram: flushing album",
		slog.String("user", first.userID),
		slog.String("chat", first.chatID),
		slog.Int("files", len(refs)))

	if err := b.dispatchFileUpdate(b.deriveContext(), first.chatID, first.userID, first.updateID, first.username, caption, refs); err != nil {
		b.logger.Error("telegram: error handling album",
			slog.String("user", first.userID),
			slog.Any("error", err))
	}
}

func (b *Bot) handleMyChatMember(c tele.Context) error {
	if b.joinHandler == nil {
		return nil
	}

	update := c.ChatMember()
	if update == nil {
		return nil
	}

	chat := update.Chat
	if chat == nil {
		return nil
	}

	chatID := strconv.FormatInt(chat.ID, 10)
	newStatus := update.NewChatMember.Role

	if newStatus == tele.Left || newStatus == tele.Kicked {
		b.logger.Info("telegram: bot removed from chat",
			slog.String("chat_id", chatID),
			slog.String("chat_type", string(chat.Type)),
			slog.String("new_status", string(newStatus)))

		ctx := b.deriveContext()
		if err := b.joinHandler.OnChatLeave(ctx, model.ChannelTelegram, chatID); err != nil {
			b.logger.Error("telegram: failed to unregister chat on leave",
				slog.String("chat_id", chatID),
				slog.Any("error", err))
		}
		return nil
	}

	var chatKind model.ChatKind
	switch chat.Type {
	case tele.ChatGroup, tele.ChatSuperGroup:
		chatKind = model.ChatKindGroup
	case tele.ChatChannel:
		chatKind = model.ChatKindChannel
	case tele.ChatPrivate:
		chatKind = model.ChatKindPrivate
	default:
		chatKind = model.ChatKindGroup
	}

	title := chat.Title
	if title == "" && chat.Type == tele.ChatPrivate {
		title = chat.FirstName
		if chat.LastName != "" {
			title += " " + chat.LastName
		}
	}

	b.logger.Info("telegram: bot added to chat",
		slog.String("chat_id", chatID),
		slog.String("chat_type", string(chat.Type)),
		slog.String("title", title),
		slog.String("new_status", string(newStatus)))

	ctx := b.deriveContext()
	if err := b.joinHandler.OnChatJoin(ctx, model.ChannelTelegram, chatID, chatKind, title); err != nil {
		b.logger.Error("telegram: failed to register chat on join",
			slog.String("chat_id", chatID),
			slog.Any("error", err))
	}
	return nil
}

func (b *Bot) registerHandlers() {

	b.bot.Handle(tele.OnText, func(c tele.Context) error {
		return b.handleTextMessage(c)
	})

	b.bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypePhoto)
	})
	b.bot.Handle(tele.OnDocument, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeDocument)
	})
	b.bot.Handle(tele.OnAudio, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeAudio)
	})
	b.bot.Handle(tele.OnVideo, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeVideo)
	})
	b.bot.Handle(tele.OnVoice, func(c tele.Context) error {
		return b.handleFileMessage(c, model.FileTypeVoice)
	})

	b.bot.Handle(tele.OnMyChatMember, func(c tele.Context) error {
		return b.handleMyChatMember(c)
	})

	b.bot.Handle(tele.OnCallback, func(c tele.Context) error {
		_ = c.Respond()

		chatID := strconv.FormatInt(c.Chat().ID, 10)
		platformUserID := strconv.FormatInt(c.Sender().ID, 10)
		updateID := strconv.Itoa(c.Update().ID)
		data := c.Callback().Data

		var callbackMsgID int
		if m := c.Callback().Message; m != nil {
			callbackMsgID = m.ID
		}

		b.logger.Info("telegram: received callback",
			slog.String("user", platformUserID),
			slog.String("chat", chatID),
			slog.String("data", data))

		ctx := b.deriveContext()
		if err := b.handler(ctx, channel.Update{
			ChannelType:      model.ChannelTelegram,
			PlatformUserID:   model.PlatformUserID(platformUserID),
			PlatformUpdateID: "tg:" + updateID,
			Input:            model.CallbackInput{Data: data, MessageID: callbackMsgID},
			ChatID:           chatID,
			Username:         c.Sender().Username,
		}); err != nil {
			b.logger.Error("telegram: error handling callback",
				slog.String("user", platformUserID),
				slog.Any("error", err))
		}
		return nil
	})
}
