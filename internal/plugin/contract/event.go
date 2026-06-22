// Package contract contains the Go-facing plugin event contract used by native
// plugins and internal trigger routing.
package contract

import (
	"encoding/json"

	"SuperBotGo/internal/model"
)

type TriggerType string

const (
	TriggerHTTP      TriggerType = "http"
	TriggerCron      TriggerType = "cron"
	TriggerEvent     TriggerType = "event"
	TriggerMessenger TriggerType = "messenger"
)

type Event struct {
	ID          string          `json:"id"`
	TriggerType TriggerType     `json:"trigger_type"`
	TriggerName string          `json:"trigger_name"`
	PluginID    string          `json:"plugin_id"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

func (e Event) Messenger() (*MessengerTriggerData, error) {
	var data MessengerTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func NewMessengerEvent(req model.CommandRequest, pluginID string) (Event, error) {
	data, err := json.Marshal(MessengerTriggerData{
		UserID:      req.UserID,
		ChannelType: req.ChannelType,
		ChatID:      req.ChatID,
		ChatGroupID: req.ChatGroupID,
		CommandName: req.CommandName,
		Params:      req.Params,
		Locale:      req.Locale,
		Files:       req.Files,
	})
	if err != nil {
		return Event{}, err
	}
	return Event{
		TriggerType: TriggerMessenger,
		TriggerName: req.CommandName,
		PluginID:    pluginID,
		Data:        data,
	}, nil
}

func (e Event) HTTP() (*HTTPTriggerData, error) {
	var data HTTPTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (e Event) Cron() (*CronTriggerData, error) {
	var data CronTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (e Event) EventTrigger() (*EventTriggerData, error) {
	var data EventTriggerData
	if err := json.Unmarshal(e.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

type ReplyBlock struct {
	Type    string            `json:"type"`
	Text    string            `json:"text,omitempty"`
	Texts   map[string]string `json:"texts,omitempty"`
	Style   string            `json:"style,omitempty"`
	UserID  string            `json:"user_id,omitempty"`
	FileID  string            `json:"file_id,omitempty"`
	Caption string            `json:"caption,omitempty"`
	URL     string            `json:"url,omitempty"`
	Label   string            `json:"label,omitempty"`
}

type EventResponse struct {
	Status      string          `json:"status,omitempty"`
	Error       string          `json:"error,omitempty"`
	ReplyBlocks []ReplyBlock    `json:"reply_blocks,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Logs        []LogEntry      `json:"logs,omitempty"`
}

type LogEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type MessengerTriggerData struct {
	UserID      model.GlobalUserID `json:"user_id"`
	ChannelType model.ChannelType  `json:"channel_type"`
	ChatID      string             `json:"chat_id"`
	ChatGroupID string             `json:"chat_group_id,omitempty"`
	CommandName string             `json:"command_name"`
	Params      model.OptionMap    `json:"params,omitempty"`
	Locale      string             `json:"locale"`
	Files       []model.FileRef    `json:"files,omitempty"`
}

type HTTPTriggerData struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
	Auth       *HTTPAuthData     `json:"auth,omitempty"`
}

type HTTPAuthKind string

const (
	HTTPAuthUser    HTTPAuthKind = "user"
	HTTPAuthService HTTPAuthKind = "service"
)

type HTTPAuthData struct {
	Kind         HTTPAuthKind       `json:"kind"`
	UserID       model.GlobalUserID `json:"user_id,omitempty"`
	ServiceKeyID int64              `json:"service_key_id,omitempty"`
}

type HTTPResponseData struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

type CronTriggerData struct {
	ScheduleName string `json:"schedule_name"`
	FireTime     int64  `json:"fire_time"`
}

type EventTriggerData struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload"`
	Source  string          `json:"source"`
}
