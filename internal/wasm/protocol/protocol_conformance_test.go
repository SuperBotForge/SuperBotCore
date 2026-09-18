package protocol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type protocolSchemaHeader struct {
	ID string `json:"$id"`
}

func TestHostProtocolDTOsConformToSchemas(t *testing.T) {
	schemas := loadProtocolSchemas(t)
	trueValue := true

	tests := []struct {
		name       string
		schemaFile string
		value      any
	}{
		{
			name:       "plugin meta",
			schemaFile: "plugin-meta.schema.json",
			value: PluginMeta{
				ID:                  "demo",
				Name:                "Demo Plugin",
				Version:             "1.0.0",
				SDKVersion:          MaxSupportedSDKVersion,
				SupportsReconfigure: true,
				RPCMethods: []RPCMethodDef{
					{Name: "lookup", Description: "Lookup demo data"},
				},
				Triggers: []TriggerDef{
					{
						Name:         "hello",
						AllowBack:    true,
						Type:         TriggerMessenger,
						Descriptions: map[string]string{"en": "Hello"},
						Nodes: []NodeDef{
							{
								Type:  "step",
								Param: "choice",
								Blocks: []BlockDef{
									{
										Type:  "text",
										Texts: map[string]string{"en": "Choose"},
										Style: "header",
									},
									{
										Type:    "options",
										Prompts: map[string]string{"en": "Choice"},
										Options: []OptionDef{
											{Label: "Yes", Labels: map[string]string{"en": "Yes"}, Value: "yes"},
										},
									},
								},
								ValidateFn: "hello:validate:choice",
								VisibleWhen: &ConditionDef{
									Param: "hidden",
									Neq:   stringPtr("true"),
								},
								Pagination: &PaginationNodeDef{
									Prompts:  map[string]string{"en": "More choices"},
									PageSize: 5,
									Provider: "hello:paginate:choice",
								},
							},
							{
								Type:    "branch",
								OnParam: "choice",
								Cases: map[string][]NodeDef{
									"yes": []NodeDef{{Type: "step", Param: "confirm"}},
								},
							},
							{
								Type: "conditional_branch",
								ConditionalCases: []CondCaseDef{
									{
										Condition: &ConditionDef{Param: "enabled", Set: &trueValue},
										Nodes:     []NodeDef{{Type: "step", Param: "enabled_path"}},
									},
								},
							},
						},
					},
					{Name: "webhook", Type: TriggerHTTP, Path: "/webhook", Methods: []string{"POST"}},
					{Name: "daily", Type: TriggerCron, Schedule: "0 8 * * *"},
					{Name: "sync", Type: TriggerEvent, Topic: "demo.sync"},
				},
				Requirements: []RequirementDef{
					{Type: "database", Description: "Store data", Name: "default"},
					{Type: "plugin", Target: "core"},
				},
				ConfigSchema: json.RawMessage(`{"type":"object","properties":{"greeting":{"type":"string"}}}`),
				Dependencies: []DependencyDef{
					{PluginID: "core", VersionConstraint: ">=1.0.0"},
				},
				Migrations: []MigrationDef{
					{Version: 1, Description: "init", Up: "CREATE TABLE demo(id text);", Down: "DROP TABLE demo;"},
				},
			},
		},
		{
			name:       "messenger event request",
			schemaFile: "event-request.schema.json",
			value: EventRequest{
				ID:          "evt-1",
				TriggerType: TriggerMessenger,
				TriggerName: "hello",
				PluginID:    "demo",
				Timestamp:   1710000000,
				Data: mustJSON(t, MessengerTriggerData{
					UserID:      42,
					ChannelType: "telegram",
					ChatID:      "chat-1",
					CommandName: "hello",
					Params:      map[string]string{"choice": "yes"},
					Locale:      "en",
					Files: []FileRef{
						{
							ID:       "file-1",
							Name:     "demo.pdf",
							MIMEType: "application/pdf",
							Size:     123,
							FileType: "document",
						},
					},
				}),
			},
		},
		{
			name:       "http event request",
			schemaFile: "event-request.schema.json",
			value: EventRequest{
				ID:          "evt-2",
				TriggerType: TriggerHTTP,
				TriggerName: "webhook",
				PluginID:    "demo",
				Timestamp:   1710000100,
				Data: mustJSON(t, HTTPTriggerData{
					Method:     "POST",
					Path:       "/webhook",
					Query:      map[string]string{"debug": "1"},
					Headers:    map[string]string{"content-type": "application/json"},
					Body:       `{"ok":true}`,
					RemoteAddr: "203.0.113.10",
					Auth:       &HTTPAuthData{Kind: "service", ServiceKeyID: 7},
				}),
			},
		},
		{
			name:       "event response",
			schemaFile: "event-response.schema.json",
			value: EventResponse{
				Status: "ok",
				ReplyBlocks: []ReplyBlock{
					{Type: "text", Text: "Done", Style: "plain"},
					{Type: "link", URL: "https://example.com", Label: "Details"},
				},
				Data: mustJSON(t, HTTPResponseData{
					StatusCode: 200,
					Headers:    map[string]string{"content-type": "application/json"},
					Body:       `{"ok":true}`,
				}),
				Logs: []LogEntry{{Level: "info", Msg: "handled"}},
			},
		},
		{
			name:       "rpc request",
			schemaFile: "rpc-request.schema.json",
			value:      RPCRequest{Caller: "caller", Method: "lookup", Params: []byte{0x81, 0xa2, 0x69, 0x64, 0x01}},
		},
		{
			name:       "rpc response",
			schemaFile: "rpc-response.schema.json",
			value:      RPCResponse{Status: "ok", Result: []byte{0x81, 0xa2, 0x6f, 0x6b, 0xc3}},
		},
		{
			name:       "step callback request",
			schemaFile: "step-callback-request.schema.json",
			value: StepCallbackRequest{
				Callback: "hello:options:choice",
				UserID:   42,
				Locale:   "en",
				Params:   map[string]string{"choice": "yes"},
				Page:     0,
				Input:    "",
			},
		},
		{
			name:       "step callback response",
			schemaFile: "step-callback-response.schema.json",
			value: StepCallbackResponse{
				Options: []OptionDef{{Label: "Yes", Labels: map[string]string{"en": "Yes"}, Value: "yes"}},
				HasMore: false,
			},
		},
		{
			name:       "reconfigure request",
			schemaFile: "reconfigure-request.schema.json",
			value: ReconfigureRequest{
				PreviousConfig: json.RawMessage(`{"greeting":"Hello"}`),
				Config:         json.RawMessage(`{"greeting":"Hi"}`),
			},
		},
		{
			name:       "migrate request",
			schemaFile: "migrate-request.schema.json",
			value:      MigrateRequest{OldVersion: "1.0.0", NewVersion: "1.1.0"},
		},
		{
			name:       "migrate response",
			schemaFile: "migrate-response.schema.json",
			value:      MigrateResponse{Status: "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("marshal %s: %v", tt.name, err)
			}
			validateProtocolJSON(t, schemas[tt.schemaFile], raw)
		})
	}
}

func loadProtocolSchemas(t *testing.T) map[string]*jsonschema.Schema {
	t.Helper()

	schemasDir := protocolSchemasDir(t)
	matches, err := filepath.Glob(filepath.Join(schemasDir, "*.schema.json"))
	if err != nil {
		t.Fatalf("glob schemas: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no schema files found in %s", schemasDir)
	}

	compiler := jsonschema.NewCompiler()
	idsByFile := make(map[string]string, len(matches))

	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read schema %s: %v", path, err)
		}

		var header protocolSchemaHeader
		if err := json.Unmarshal(raw, &header); err != nil {
			t.Fatalf("parse schema header %s: %v", path, err)
		}
		if header.ID == "" {
			t.Fatalf("schema %s has no $id", path)
		}

		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("unmarshal schema %s: %v", path, err)
		}
		if err := compiler.AddResource(header.ID, doc); err != nil {
			t.Fatalf("add schema resource %s: %v", path, err)
		}
		idsByFile[filepath.Base(path)] = header.ID
	}

	compiled := make(map[string]*jsonschema.Schema, len(idsByFile))
	for file, id := range idsByFile {
		schema, err := compiler.Compile(id)
		if err != nil {
			t.Fatalf("compile schema %s: %v", file, err)
		}
		compiled[file] = schema
	}
	return compiled
}

func protocolSchemasDir(t *testing.T) string {
	t.Helper()

	if fromEnv := os.Getenv("SUPERBOTGO_PROTOCOL_SCHEMAS"); fromEnv != "" {
		if dirExists(fromEnv) {
			return fromEnv
		}
		t.Fatalf("SUPERBOTGO_PROTOCOL_SCHEMAS=%q does not exist", fromEnv)
	}

	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("locate current test file")
	}
	candidates := []string{
		filepath.Join(filepath.Dir(filename), "..", "..", "..", "sdk", "protocol", "v4", "schemas"),
		filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "sdk", "protocol", "v4", "schemas"),
	}
	for _, dir := range candidates {
		if dirExists(dir) {
			return dir
		}
	}

	t.Skip("protocol schemas not found; set SUPERBOTGO_PROTOCOL_SCHEMAS or clone github.com/SuperBotForge/sdk next to SuperBotGo")
	return ""
}

func validateProtocolJSON(t *testing.T, schema *jsonschema.Schema, data []byte) {
	t.Helper()
	if schema == nil {
		t.Fatal("schema is nil")
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal protocol JSON %s: %v", data, err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("protocol JSON does not conform:\n%s\nerror: %v", data, err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal test JSON: %v", err)
	}
	return raw
}

func stringPtr(v string) *string {
	return &v
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
