// Package protocol contains the core-side Go representation of the SuperBotGo
// WASM plugin wire protocol.
package protocol

import "encoding/json"

const MaxSupportedSDKVersion = 4

const ActionMigrate = "migrate"
const ActionReconfigure = "reconfigure"
const ActionHandleRPC = "handle_rpc"

const TriggerHTTP = "http"
const TriggerCron = "cron"
const TriggerEvent = "event"
const TriggerMessenger = "messenger"

type PluginMeta struct {
	ID                  string           `json:"id"`
	Name                string           `json:"name"`
	Version             string           `json:"version"`
	SDKVersion          int              `json:"sdk_version"`
	SupportsReconfigure bool             `json:"supports_reconfigure,omitempty"`
	SupportsVisibility  bool             `json:"supports_visibility,omitempty"`
	RPCMethods          []RPCMethodDef   `json:"rpc_methods,omitempty"`
	Triggers            []TriggerDef     `json:"triggers,omitempty"`
	Requirements        []RequirementDef `json:"requirements,omitempty"`
	ConfigSchema        json.RawMessage  `json:"config_schema,omitempty"`
	Dependencies        []DependencyDef  `json:"dependencies,omitempty"`
	Migrations          []MigrationDef   `json:"migrations,omitempty"`
}

type ReconfigureRequest struct {
	PreviousConfig json.RawMessage `json:"previous_config,omitempty"`
	Config         json.RawMessage `json:"config,omitempty"`
}

type MigrateRequest struct {
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

type MigrateResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type RPCMethodDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RPCRequest struct {
	Caller string `json:"caller,omitempty"`
	Method string `json:"method"`
	Params []byte `json:"params,omitempty"`
}

type RPCResponse struct {
	Status string `json:"status"`
	Result []byte `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type DependencyDef struct {
	PluginID          string `json:"plugin"`
	VersionConstraint string `json:"version"`
}

// MigrationDef describes a single SQL migration declared by a plugin.
type MigrationDef struct {
	Version     int    `json:"version"`
	Description string `json:"description"`
	Up          string `json:"up"`
	Down        string `json:"down,omitempty"`
}

type TriggerDef struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Descriptions map[string]string `json:"descriptions,omitempty"`
	Description  string            `json:"description,omitempty"` // Deprecated: use Descriptions for user-facing trigger text.
	Path         string            `json:"path,omitempty"`
	Methods      []string          `json:"methods,omitempty"`
	Schedule     string            `json:"schedule,omitempty"`
	Topic        string            `json:"topic,omitempty"`
	Nodes        []NodeDef         `json:"nodes,omitempty"`
}

type OptionDef struct {
	Label  string            `json:"label"`
	Labels map[string]string `json:"labels,omitempty"`
	Value  string            `json:"value"`
}

type RequirementDef struct {
	Type        string          `json:"type"`
	Description string          `json:"description,omitempty"`
	Name        string          `json:"name,omitempty"`
	Target      string          `json:"target,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
}

type NodeDef struct {
	Type             string               `json:"type"`
	Param            string               `json:"param,omitempty"`
	Blocks           []BlockDef           `json:"blocks,omitempty"`
	Validation       string               `json:"validation,omitempty"`
	ValidateFn       string               `json:"validate_fn,omitempty"`
	VisibleWhen      *ConditionDef        `json:"visible_when,omitempty"`
	ConditionFn      string               `json:"condition_fn,omitempty"`
	Pagination       *PaginationNodeDef   `json:"pagination,omitempty"`
	OnParam          string               `json:"on_param,omitempty"`
	Cases            map[string][]NodeDef `json:"cases,omitempty"`
	ConditionalCases []CondCaseDef        `json:"conditional_cases,omitempty"`
	Default          []NodeDef            `json:"default,omitempty"`
}

type BlockDef struct {
	Type      string            `json:"type"`
	Text      string            `json:"text,omitempty"`
	Texts     map[string]string `json:"texts,omitempty"`
	Style     string            `json:"style,omitempty"`
	Prompt    string            `json:"prompt,omitempty"`
	Prompts   map[string]string `json:"prompts,omitempty"`
	Options   []OptionDef       `json:"options,omitempty"`
	OptionsFn string            `json:"options_fn,omitempty"`
	URL       string            `json:"url,omitempty"`
	Label     string            `json:"label,omitempty"`
}

type ConditionDef struct {
	Param string          `json:"param,omitempty"`
	Eq    *string         `json:"eq,omitempty"`
	Neq   *string         `json:"neq,omitempty"`
	Match string          `json:"match,omitempty"`
	Set   *bool           `json:"set,omitempty"`
	And   []*ConditionDef `json:"and,omitempty"`
	Or    []*ConditionDef `json:"or,omitempty"`
	Not   *ConditionDef   `json:"not,omitempty"`
}

type PaginationNodeDef struct {
	Prompt   string            `json:"prompt"`
	Prompts  map[string]string `json:"prompts,omitempty"`
	PageSize int               `json:"page_size"`
	Provider string            `json:"provider"`
}

type CondCaseDef struct {
	Condition   *ConditionDef `json:"condition,omitempty"`
	ConditionFn string        `json:"condition_fn,omitempty"`
	Nodes       []NodeDef     `json:"nodes"`
}

type StepCallbackRequest struct {
	Callback string            `json:"callback"`
	UserID   int64             `json:"user_id"`
	Locale   string            `json:"locale"`
	Params   map[string]string `json:"params"`
	Page     int               `json:"page"`
	Input    string            `json:"input"`
}

type StepCallbackResponse struct {
	Options []OptionDef `json:"options,omitempty"`
	HasMore bool        `json:"has_more,omitempty"`
	Result  *bool       `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type EventRequest struct {
	ID          string          `json:"id"`
	TriggerType string          `json:"trigger_type"`
	TriggerName string          `json:"trigger_name"`
	PluginID    string          `json:"plugin_id"`
	Timestamp   int64           `json:"timestamp"`
	Data        json.RawMessage `json:"data"`
}

type EventResponse struct {
	Status      string          `json:"status,omitempty"`
	Error       string          `json:"error,omitempty"`
	ReplyBlocks []ReplyBlock    `json:"reply_blocks,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Logs        []LogEntry      `json:"logs,omitempty"`
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

type LogEntry struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type MessengerTriggerData struct {
	UserID      int64             `json:"user_id"`
	ChannelType string            `json:"channel_type"`
	ChatID      string            `json:"chat_id"`
	ChatGroupID string            `json:"chat_group_id,omitempty"`
	CommandName string            `json:"command_name"`
	Params      map[string]string `json:"params,omitempty"`
	Locale      string            `json:"locale"`
	Files       []FileRef         `json:"files,omitempty"`
}

type FileRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type"`
	Size     int64  `json:"size"`
	FileType string `json:"file_type"`
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

type HTTPAuthData struct {
	Kind         string `json:"kind"`
	UserID       int64  `json:"user_id,omitempty"`
	ServiceKeyID int64  `json:"service_key_id,omitempty"`
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
