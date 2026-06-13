package hostapi

// RequirementType describes a type of resource a plugin can require.
type RequirementType struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	HasConfig   bool   `json:"has_config"`
}

// AllRequirementTypes returns the list of supported requirement types.
func AllRequirementTypes() []RequirementType {
	return []RequirementType{
		{Type: "database", Description: "SQL database access (PostgreSQL)", HasConfig: true},
		{Type: "http", Description: "Outbound HTTP requests", HasConfig: false},
		{Type: "kv", Description: "Key-value store", HasConfig: false},
		{Type: "notify", Description: "Send notifications", HasConfig: false},
		{Type: "events", Description: "Publish events", HasConfig: false},
		{Type: "plugin", Description: "Call another plugin", HasConfig: false},
		{Type: "file", Description: "File store access (read, write, metadata)", HasConfig: false},
		{Type: "user_info", Description: "Fetch user information by ID", HasConfig: false},
	}
}
