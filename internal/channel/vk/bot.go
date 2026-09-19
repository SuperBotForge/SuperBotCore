package vk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"SuperBotGo/internal/channel"
	"SuperBotGo/internal/filestore"
	"SuperBotGo/internal/model"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
	"github.com/SevereCloud/vksdk/v3/callback"
	"github.com/SevereCloud/vksdk/v3/events"
	longpoll "github.com/SevereCloud/vksdk/v3/longpoll-bot"
	vkobject "github.com/SevereCloud/vksdk/v3/object"
)

type BotConfig struct {
	Token        string
	Mode         string
	CallbackURL  string
	CallbackPath string
}

type Bot struct {
	vk              *vkapi.VK
	callbackHandler *callback.Callback
	mode            string
	callbackURL     string
	callbackPath    string
	handler         channel.UpdateHandlerFunc
	joinHandler     channel.ChatJoinHandler
	fileStore       filestore.FileStore
	httpClient      *http.Client
	maxFileSize     int64
	logger          *slog.Logger
	connected       atomic.Bool
	lifecycleCtx    context.Context
	knownChats      sync.Map
}

func NewBot(cfg BotConfig, handler channel.UpdateHandlerFunc, joinHandler channel.ChatJoinHandler, fs filestore.FileStore, maxFileSize int64, logger *slog.Logger) (*Bot, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("vk: token is required")
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "longpoll"
	}
	callbackPath := cfg.CallbackPath
	if callbackPath == "" {
		callbackPath = "/vk/callback"
	}

	vk := vkapi.NewVK(cfg.Token)
	switch mode {
	case "longpoll":
		if _, err := longpoll.NewLongPollCommunity(vk); err != nil {
			return nil, fmt.Errorf("vk: initialise longpoll: %w", err)
		}
	case "callback":
		if cfg.CallbackURL == "" {
			return nil, fmt.Errorf("vk: callback_url is required in callback mode")
		}
		if _, err := vk.GroupsGetByID(vkapi.Params{}); err != nil {
			return nil, fmt.Errorf("vk: validate callback token: %w", err)
		}
	default:
		return nil, fmt.Errorf("vk: unsupported mode %q", mode)
	}

	var cb *callback.Callback
	if mode == "callback" {
		cb = callback.NewCallback()
		cb.ErrorSLog = logger
	}

	bot := &Bot{
		vk:              vk,
		callbackHandler: cb,
		mode:            mode,
		callbackURL:     cfg.CallbackURL,
		callbackPath:    callbackPath,
		handler:         handler,
		joinHandler:     joinHandler,
		fileStore:       fs,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		maxFileSize:     maxFileSize,
		logger:          logger,
	}
	if bot.callbackHandler != nil {
		bot.registerCallbackHandlers(bot.callbackHandler)
	}

	return bot, nil
}

func (b *Bot) Adapter() *Adapter {
	return NewAdapter(b.vk, &b.connected, b.fileStore)
}

func (b *Bot) RegisterRoutes(mux *http.ServeMux) error {
	if b.mode != "callback" {
		return nil
	}
	if mux == nil {
		return fmt.Errorf("vk: mux is nil")
	}
	if b.callbackHandler == nil {
		return fmt.Errorf("vk: callback handler is not initialized")
	}

	mux.HandleFunc(b.callbackPath, b.callbackHandler.HandleFunc)
	return nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.lifecycleCtx = ctx
	if b.mode == "callback" {
		return b.startCallback(ctx)
	}

	return b.startLongPoll(ctx)
}

// deriveContext returns a context derived from the bot lifecycle context.
func (b *Bot) deriveContext() context.Context {
	if b.lifecycleCtx != nil {
		return b.lifecycleCtx
	}
	return context.Background()
}

func (b *Bot) startLongPoll(ctx context.Context) error {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			b.connected.Store(false)
			return nil
		}

		lp, err := longpoll.NewLongPollCommunity(b.vk)
		if err != nil {
			b.connected.Store(false)
			b.logger.Error("vk: failed to initialise longpoll", slog.Any("error", err))
			if waitErr := sleepWithContext(ctx, backoff); waitErr != nil {
				return nil
			}
			backoff = nextBackoff(backoff)
			continue
		}
		b.registerHandlers(lp)

		b.connected.Store(true)
		b.logger.Info("VK bot starting", slog.String("mode", b.mode))
		err = lp.RunWithContext(ctx)
		b.connected.Store(false)
		if ctx.Err() != nil {
			return nil
		}

		b.logger.Error("vk: longpoll stopped", slog.Any("error", err))
		if waitErr := sleepWithContext(ctx, backoff); waitErr != nil {
			return nil
		}
		backoff = nextBackoff(backoff)
	}
}

func (b *Bot) startCallback(ctx context.Context) error {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			b.connected.Store(false)
			return nil
		}

		if err := b.callbackHandler.AutoSetting(b.vk, b.callbackURL); err != nil {
			b.connected.Store(false)
			b.logger.Error("vk: callback auto-setup failed",
				slog.String("callback_url", b.callbackURL),
				slog.Any("error", err))
			if waitErr := sleepWithContext(ctx, backoff); waitErr != nil {
				return nil
			}
			backoff = nextBackoff(backoff)
			continue
		}

		b.connected.Store(true)
		b.logger.Info("VK bot starting",
			slog.String("mode", b.mode),
			slog.String("callback_path", b.callbackPath),
			slog.String("callback_url", b.callbackURL))
		<-ctx.Done()
		b.connected.Store(false)
		return nil
	}
}

func (b *Bot) registerHandlers(lp *longpoll.LongPoll) {
	lp.MessageEvent(b.handleMessageEvent)
	lp.MessageNew(func(ctx context.Context, obj events.MessageNewObject) {
		b.handleMessageNew(ctx, obj)
	})
}

func (b *Bot) registerCallbackHandlers(cb *callback.Callback) {
	cb.MessageEvent(b.handleMessageEvent)
	cb.MessageNew(func(ctx context.Context, obj events.MessageNewObject) {
		b.handleMessageNew(ctx, obj)
	})
}

// handleMessageEvent handles silent callback buttons in both delivery modes.
func (b *Bot) handleMessageEvent(_ context.Context, obj events.MessageEventObject) {
	if obj.UserID <= 0 || obj.PeerID <= 0 || obj.EventID == "" {
		return
	}
	ctx := b.deriveContext()
	if _, err := b.vk.MessagesSendMessageEventAnswer(vkapi.Params{
		"event_id": obj.EventID, "user_id": obj.UserID, "peer_id": obj.PeerID,
	}.WithContext(ctx)); err != nil {
		b.logger.Warn("vk: failed to acknowledge button", slog.Any("error", err))
	}
	data := parsePayloadValue(string(obj.Payload))
	if data == "" {
		return
	}
	b.ensureChatRegistered(ctx, obj.PeerID)
	if err := b.handler(ctx, channel.Update{
		ChannelType:      model.ChannelVK,
		PlatformUserID:   model.PlatformUserID(strconv.Itoa(obj.UserID)),
		PlatformUpdateID: fmt.Sprintf("vk:button:%d:%d:%s", obj.PeerID, obj.UserID, obj.EventID),
		ChatID:           strconv.Itoa(obj.PeerID),
		Input:            model.CallbackInput{Data: data},
	}); err != nil {
		b.logger.Error("vk: error handling button", slog.Any("error", err))
	}
}

func (b *Bot) handleMessageNew(ctx context.Context, obj events.MessageNewObject) {
	msg := obj.Message
	if bool(msg.Out) || msg.FromID <= 0 {
		return
	}

	chatID := strconv.Itoa(msg.PeerID)
	platformUserID := strconv.Itoa(msg.FromID)
	updateID := eventIDFromContext(ctx)
	if updateID == "" {
		updateID = fmt.Sprintf("%d:%d", msg.PeerID, msg.ID)
	}

	processingCtx := b.deriveContext()

	b.ensureChatRegistered(processingCtx, msg.PeerID)

	input, ok := b.buildInput(processingCtx, msg)
	if !ok {
		return
	}

	if err := b.handler(processingCtx, channel.Update{
		ChannelType:      model.ChannelVK,
		PlatformUserID:   model.PlatformUserID(platformUserID),
		PlatformUpdateID: "vk:" + updateID,
		Input:            input,
		ChatID:           chatID,
	}); err != nil {
		b.logger.Error("vk: error handling message",
			slog.String("user", platformUserID),
			slog.String("chat", chatID),
			slog.Any("error", err))
	}
}

func eventIDFromContext(ctx context.Context) (eventID string) {
	defer func() {
		if recover() != nil {
			eventID = ""
		}
	}()
	return events.EventIDFromContext(ctx)
}

func (b *Bot) buildInput(ctx context.Context, msg vkobject.MessagesMessage) (model.UserInput, bool) {
	if callbackData := parsePayloadValue(msg.Payload); callbackData != "" {
		return model.CallbackInput{Data: callbackData, Label: msg.Text}, true
	}

	if refs := b.collectFiles(ctx, msg.Attachments); len(refs) > 0 {
		return model.FileInput{Caption: msg.Text, Files: refs}, true
	}

	if isVKStartButtonMessage(msg) {
		return model.TextInput{Text: "/start"}, true
	}

	if strings.TrimSpace(msg.Text) == "" {
		return nil, false
	}

	return model.TextInput{Text: msg.Text}, true
}

func isVKStartButtonMessage(msg vkobject.MessagesMessage) bool {
	if msg.PeerID != msg.FromID {
		return false
	}
	if msg.ConversationMessageID != 1 {
		return false
	}
	if strings.TrimSpace(msg.Payload) != "" {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(msg.Text)) {
	case "начать", "start":
		return true
	default:
		return false
	}
}

func (b *Bot) collectFiles(ctx context.Context, attachments []vkobject.MessagesMessageAttachment) []model.FileRef {
	if b.fileStore == nil || len(attachments) == 0 {
		return nil
	}

	refs := make([]model.FileRef, 0, len(attachments))
	for _, att := range attachments {
		ref, err := b.downloadAttachment(ctx, att)
		if err != nil {
			b.logger.Error("vk: failed to download attachment", slog.Any("error", err))
			continue
		}
		if ref.ID != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func (b *Bot) downloadAttachment(ctx context.Context, att vkobject.MessagesMessageAttachment) (model.FileRef, error) {
	var (
		url      string
		name     string
		size     int64
		fileType model.FileType
	)

	switch att.Type {
	case "photo":
		url = att.Photo.MaxSize().URL
		name = fallbackFileName(url, "photo.jpg")
		fileType = model.FileTypePhoto
	case "doc":
		url = att.Doc.URL
		name = fallbackFileName(url, att.Doc.Title)
		size = int64(att.Doc.Size)
	case "audio_message":
		url = att.AudioMessage.URL
		name = fallbackFileName(url, "voice.ogg")
		size = int64(att.AudioMessage.Size)
		fileType = model.FileTypeVoice
	case "graffiti":
		url = att.Graffiti.URL
		name = fallbackFileName(url, "graffiti.png")
		size = int64(att.Graffiti.Size)
		fileType = model.FileTypeSticker
	default:
		return model.FileRef{}, nil
	}

	if url == "" {
		return model.FileRef{}, nil
	}
	if b.maxFileSize > 0 && size > 0 && size > b.maxFileSize {
		b.logger.Warn("vk: attachment too large, skipping",
			slog.String("name", name),
			slog.Int64("size", size),
			slog.Int64("max_size", b.maxFileSize))
		return model.FileRef{}, nil
	}

	resp, err := b.httpClient.Get(url)
	if err != nil {
		return model.FileRef{}, fmt.Errorf("vk: download %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.FileRef{}, fmt.Errorf("vk: download %q: status %d", url, resp.StatusCode)
	}

	mimeType := resp.Header.Get("Content-Type")
	fileType = inferFileType(name, mimeType, fileType)

	finalSize := size
	if finalSize <= 0 && resp.ContentLength > 0 {
		finalSize = resp.ContentLength
	}
	if b.maxFileSize > 0 && finalSize > 0 && finalSize > b.maxFileSize {
		return model.FileRef{}, nil
	}

	return storeDownloadedFile(ctx, b.fileStore, filestore.FileMeta{
		Name:     name,
		MIMEType: mimeType,
		Size:     finalSize,
		FileType: fileType,
	}, resp.Body)
}

func (b *Bot) ensureChatRegistered(ctx context.Context, peerID int) {
	if b.joinHandler == nil {
		return
	}

	key := strconv.Itoa(peerID)
	if _, loaded := b.knownChats.LoadOrStore(key, struct{}{}); loaded {
		return
	}

	kind, title := b.resolveChatMeta(ctx, peerID)
	if err := b.joinHandler.OnChatJoin(ctx, model.ChannelVK, key, kind, title); err != nil {
		b.knownChats.Delete(key)
		b.logger.Error("vk: failed to register chat",
			slog.String("chat", key),
			slog.Any("error", err))
	}
}

func (b *Bot) resolveChatMeta(ctx context.Context, peerID int) (model.ChatKind, string) {
	if peerID > 2_000_000_000 {
		resp, err := b.vk.MessagesGetConversationsByID(vkapi.Params{
			"peer_ids": []int{peerID},
		}.WithContext(ctx))
		if err == nil && len(resp.Items) > 0 && resp.Items[0].ChatSettings.Title != "" {
			return model.ChatKindGroup, resp.Items[0].ChatSettings.Title
		}
		return model.ChatKindGroup, fmt.Sprintf("VK chat %d", peerID-2_000_000_000)
	}

	return model.ChatKindPrivate, fmt.Sprintf("VK user %d", peerID)
}

func parsePayloadValue(payload string) string {
	if payload == "" {
		return ""
	}

	var raw string
	if err := json.Unmarshal([]byte(payload), &raw); err == nil {
		return raw
	}

	var data buttonPayload
	if err := json.Unmarshal([]byte(payload), &data); err == nil {
		return data.Value
	}

	return ""
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextBackoff(current time.Duration) time.Duration {
	if current >= 30*time.Second {
		return 30 * time.Second
	}
	current *= 2
	if current > 30*time.Second {
		return 30 * time.Second
	}
	return current
}
