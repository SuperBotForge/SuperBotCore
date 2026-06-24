package channel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"SuperBotGo/internal/metrics"
	"SuperBotGo/internal/model"
)

var ErrNoAdapter = errors.New("no adapter registered for channel type")

type AdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[model.ChannelType]ChannelAdapter
	metrics  *metrics.Metrics
}

func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[model.ChannelType]ChannelAdapter),
	}
}

func (r *AdapterRegistry) Register(adapter ChannelAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Type()] = adapter
}

func (r *AdapterRegistry) Get(channelType model.ChannelType) ChannelAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType]
}

func (r *AdapterRegistry) SetMetrics(m *metrics.Metrics) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = m
}

func (r *AdapterRegistry) mustGet(channelType model.ChannelType) (ChannelAdapter, error) {
	adapter := r.Get(channelType)
	if adapter == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoAdapter, channelType)
	}
	return adapter, nil
}

// IsRegistered reports whether an adapter for the given channel type exists.
func (r *AdapterRegistry) IsRegistered(channelType model.ChannelType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelType] != nil
}

// SendToChat dispatches a message to the appropriate adapter with retry on transient errors.
func (r *AdapterRegistry) SendToChat(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message) error {
	start := time.Now()
	adapter, err := r.mustGet(channelType)
	if err != nil {
		r.observeSend(channelType, "chat", classifySendResult(err), time.Since(start))
		return err
	}
	err = withRetry(ctx, func() error {
		return adapter.SendToChat(ctx, chatID, msg)
	}, func() {
		r.incSendRetry(channelType, "chat")
	})
	r.observeSend(channelType, "chat", classifySendResult(err), time.Since(start))
	return err
}

// SendToUser dispatches a message to the appropriate adapter with retry on transient errors.
func (r *AdapterRegistry) SendToUser(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message) error {
	start := time.Now()
	adapter, err := r.mustGet(channelType)
	if err != nil {
		r.observeSend(channelType, "user", classifySendResult(err), time.Since(start))
		return err
	}
	err = withRetry(ctx, func() error {
		return adapter.SendToUser(ctx, platformUserID, msg)
	}, func() {
		r.incSendRetry(channelType, "user")
	})
	r.observeSend(channelType, "user", classifySendResult(err), time.Since(start))
	return err
}

// sendWithOpts applies SendOptions (silent mode, mention stripping) and dispatches
// with retry on transient errors. normalSend and silentSend are the platform-specific senders.
func sendWithOpts(ctx context.Context, adapter ChannelAdapter, msg model.Message, opts model.SendOptions, normalSend, silentSend func(model.Message) error) error {
	if opts.StripMentions {
		msg = model.StripMentionBlocks(msg)
	}
	return withRetry(ctx, func() error {
		if opts.Silent {
			if _, ok := adapter.(SilentSender); ok {
				return silentSend(msg)
			}
		}
		return normalSend(msg)
	}, nil)
}

// SendToChatWithOpts dispatches a message applying SendOptions (silent mode, mention stripping)
// with retry on transient errors.
func (r *AdapterRegistry) SendToChatWithOpts(ctx context.Context, channelType model.ChannelType, chatID string, msg model.Message, opts model.SendOptions) error {
	start := time.Now()
	adapter, err := r.mustGet(channelType)
	if err != nil {
		r.observeSend(channelType, "chat", classifySendResult(err), time.Since(start))
		return err
	}
	err = withRetry(ctx, func() error {
		if opts.StripMentions {
			msg = model.StripMentionBlocks(msg)
		}
		if opts.Silent {
			if _, ok := adapter.(SilentSender); ok {
				return adapter.(SilentSender).SendToChatSilent(ctx, chatID, msg, true)
			}
		}
		return adapter.SendToChat(ctx, chatID, msg)
	}, func() {
		r.incSendRetry(channelType, "chat")
	})
	r.observeSend(channelType, "chat", classifySendResult(err), time.Since(start))
	return err
}

// SendToUserWithOpts dispatches a message applying SendOptions (silent mode, mention stripping)
// with retry on transient errors.
func (r *AdapterRegistry) SendToUserWithOpts(ctx context.Context, channelType model.ChannelType, platformUserID model.PlatformUserID, msg model.Message, opts model.SendOptions) error {
	start := time.Now()
	adapter, err := r.mustGet(channelType)
	if err != nil {
		r.observeSend(channelType, "user", classifySendResult(err), time.Since(start))
		return err
	}
	err = withRetry(ctx, func() error {
		if opts.StripMentions {
			msg = model.StripMentionBlocks(msg)
		}
		if opts.Silent {
			if _, ok := adapter.(SilentSender); ok {
				return adapter.(SilentSender).SendToUserSilent(ctx, platformUserID, msg, true)
			}
		}
		return adapter.SendToUser(ctx, platformUserID, msg)
	}, func() {
		r.incSendRetry(channelType, "user")
	})
	r.observeSend(channelType, "user", classifySendResult(err), time.Since(start))
	return err
}

// EditMessageInChat edits a previously sent message in place if the adapter supports it.
// Pass an empty msg to remove the inline keyboard. Returns nil if the adapter does not
// implement MessageEditor (edit is a no-op in that case).
func (r *AdapterRegistry) EditMessageInChat(ctx context.Context, channelType model.ChannelType, chatID string, messageID int, msg model.Message) error {
	adapter, err := r.mustGet(channelType)
	if err != nil {
		return err
	}
	editor, ok := adapter.(MessageEditor)
	if !ok {
		return nil
	}
	return editor.EditMessage(ctx, chatID, messageID, msg)
}

func (r *AdapterRegistry) observeSend(channelType model.ChannelType, target, result string, dur time.Duration) {
	if r.metrics == nil {
		return
	}
	r.metrics.MessageSendDuration.WithLabelValues(string(channelType), target, result).Observe(dur.Seconds())
}

func (r *AdapterRegistry) incSendRetry(channelType model.ChannelType, target string) {
	if r.metrics == nil {
		return
	}
	r.metrics.MessageSendRetriesTotal.WithLabelValues(string(channelType), target).Inc()
}

func classifySendResult(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrNoAdapter):
		return "no_adapter"
	default:
		return "error"
	}
}
