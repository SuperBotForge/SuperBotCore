package hostapi

import (
	"context"
	"fmt"
	"time"

	"SuperBotGo/internal/model"
	wasmrt "SuperBotGo/internal/wasm/runtime"

	"github.com/tetratelabs/wazero/api"
	"github.com/vmihailenco/msgpack/v5"
)

var wasmNotifyMaxTimeout = time.Duration(wasmrt.DefaultHostNotifyTimeoutSeconds) * time.Second

type notifyUserRequest struct {
	UserID   int64  `msgpack:"user_id"`
	Text     string `msgpack:"text"`
	Priority int    `msgpack:"priority"`
}

type notifyResponse struct {
	OK    bool   `msgpack:"ok"`
	Error string `msgpack:"error,omitempty"`
}

func (h *HostAPI) notifyUserFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyUserRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyUser(reqCtx, req.UserID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

type notifyChatRequest struct {
	ChannelType string `msgpack:"channel_type"`
	ChatID      string `msgpack:"chat_id"`
	Text        string `msgpack:"text"`
	Priority    int    `msgpack:"priority"`
}

func (h *HostAPI) notifyChatFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyChatRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		if err := h.deps.Notifier.NotifyChat(reqCtx, req.ChannelType, req.ChatID, req.Text, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

type notifyUsersRequest struct {
	UserIDs  []int64     `msgpack:"user_ids"`
	Blocks   []wireBlock `msgpack:"blocks"`
	Priority int         `msgpack:"priority"`
}

func (h *HostAPI) notifyUsersFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyUsersRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)
		msg, err := wireBlocksToMessage(req.Blocks)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.deps.Notifier.NotifyUsers(reqCtx, req.UserIDs, msg, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

type notifyTeacherRequest struct {
	TeacherPositionID int64       `msgpack:"teacher_position_id,omitempty"`
	PersonID          int64       `msgpack:"person_id,omitempty"`
	ExternalID        string      `msgpack:"external_id,omitempty"`
	Blocks            []wireBlock `msgpack:"blocks"`
	Priority          int         `msgpack:"priority"`
}

func (h *HostAPI) notifyTeacherFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyTeacherRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)
		msg, err := wireBlocksToMessage(req.Blocks)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		ref := model.TeacherRef{
			TeacherPositionID: req.TeacherPositionID,
			PersonID:          req.PersonID,
			ExternalID:        req.ExternalID,
		}
		if err := h.deps.Notifier.NotifyTeacher(reqCtx, ref, msg, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

type wireBlockOption struct {
	Label string `msgpack:"label"`
	Value string `msgpack:"value"`
}

type wireBlock struct {
	Type    string            `msgpack:"type"`
	Text    string            `msgpack:"text,omitempty"`
	Style   string            `msgpack:"style,omitempty"`
	UserID  string            `msgpack:"user_id,omitempty"`
	FileID  string            `msgpack:"file_id,omitempty"`
	Caption string            `msgpack:"caption,omitempty"`
	URL     string            `msgpack:"url,omitempty"`
	Label   string            `msgpack:"label,omitempty"`
	Prompt  string            `msgpack:"prompt,omitempty"`
	Options []wireBlockOption `msgpack:"options,omitempty"`
}

type notifyStudentsRequest struct {
	Scope    string      `msgpack:"scope"`
	TargetID int64       `msgpack:"target_id"`
	Blocks   []wireBlock `msgpack:"blocks"`
	Priority int         `msgpack:"priority"`
}

func (h *HostAPI) notifyStudentsFunc() api.GoModuleFunc {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		pluginID := pluginIDFromContext(ctx)
		offset := uint32(stack[0])
		length := uint32(stack[1])

		data, err := readPayload(mod, offset, length)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		var req notifyStudentsRequest
		if err := msgpack.Unmarshal(data, &req); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.perms.CheckPermission(pluginID, "notify"); err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if h.deps.Notifier == nil {
			returnError(ctx, mod, stack, errDepNotAvailable("Notifier"))
			return
		}

		reqCtx, cancel := context.WithTimeout(ctx, contextAwareTimeout(ctx, wasmNotifyMaxTimeout))
		defer cancel()

		priority := clampPriority(req.Priority)

		msg, err := wireBlocksToMessage(req.Blocks)
		if err != nil {
			returnError(ctx, mod, stack, err)
			return
		}

		if err := h.deps.Notifier.NotifyStudents(reqCtx, req.Scope, req.TargetID, msg, priority); err != nil {
			SetHostCallStatus(ctx, "error")
			writeResult(ctx, mod, stack, notifyResponse{OK: false, Error: err.Error()})
			return
		}

		writeResult(ctx, mod, stack, notifyResponse{OK: true})
	}
}

var wireStyleToModel = map[string]model.TextStyle{
	"plain":     model.StylePlain,
	"header":    model.StyleHeader,
	"subheader": model.StyleSubheader,
	"code":      model.StyleCode,
	"quote":     model.StyleQuote,
}

func wireBlocksToMessage(blocks []wireBlock) (model.Message, error) {
	if len(blocks) == 0 {
		return model.Message{}, fmt.Errorf("empty message blocks")
	}

	content := make([]model.ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			style := wireStyleToModel[b.Style]
			content = append(content, model.TextBlock{Text: b.Text, Style: style})
		case "mention":
			content = append(content, model.MentionBlock{UserID: b.UserID})
		case "file":
			content = append(content, model.FileBlock{
				FileRef: model.FileRef{ID: b.FileID},
				Caption: b.Caption,
			})
		case "options":
			opts := make([]model.Option, 0, len(b.Options))
			for _, o := range b.Options {
				opts = append(opts, model.Option{Label: o.Label, Value: o.Value})
			}
			content = append(content, model.OptionsBlock{Prompt: b.Prompt, Options: opts})
		case "link":
			content = append(content, model.LinkBlock{URL: b.URL, Label: b.Label})
		case "image":
			content = append(content, model.ImageBlock{URL: b.URL})
		default:
			return model.Message{}, fmt.Errorf("unknown block type: %q", b.Type)
		}
	}
	return model.Message{Blocks: content}, nil
}

func clampPriority(p int) int {
	if p < 0 {
		return 0
	}
	if p > 3 {
		return 3
	}
	return p
}
