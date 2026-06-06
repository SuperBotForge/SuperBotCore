package hostapi

import (
	"fmt"
	"sync"

	wasmrt "SuperBotGo/internal/wasm/runtime"
)

// PermissionsFromRequirements derives flat internal permission strings
// from plugin requirements. This is the bridge between the new Requirements
// system (what the plugin developer sees) and the internal permission checks.
func PermissionsFromRequirements(reqs []wasmrt.RequirementDef) []string {
	var perms []string
	for _, r := range reqs {
		switch r.Type {
		case "database":
			perms = append(perms, "sql")
		case "http":
			perms = append(perms, "network")
		case "kv":
			perms = append(perms, "kv")
		case "notify":
			perms = append(perms, "notify")
		case "events":
			perms = append(perms, "events")
		case "file":
			perms = append(perms, "file")
		case "user_info":
			perms = append(perms, "user_info")
		case "plugin":
			if r.Target != "" {
				perms = append(perms, "plugins:call:"+r.Target)
			}
		}
	}
	return perms
}

var ErrPermissionDenied = fmt.Errorf("permission denied")

type permissionStore struct {
	mu    sync.RWMutex
	perms map[string]map[string]bool
}

func newPermissionStore() *permissionStore {
	return &permissionStore{
		perms: make(map[string]map[string]bool),
	}
}

func (ps *permissionStore) Grant(pluginID string, permissions []string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	set := make(map[string]bool, len(permissions))
	for _, p := range permissions {
		set[p] = true
	}
	ps.perms[pluginID] = set
}

func (ps *permissionStore) Revoke(pluginID string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.perms, pluginID)
}

func (ps *permissionStore) CheckPermission(pluginID, requiredPermission string) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	perms, ok := ps.perms[pluginID]
	if !ok {
		return fmt.Errorf("%w: plugin %q has no registered permissions", ErrPermissionDenied, pluginID)
	}
	if !perms[requiredPermission] {
		return fmt.Errorf("%w: plugin %q lacks permission %q", ErrPermissionDenied, pluginID, requiredPermission)
	}
	return nil
}

func (ps *permissionStore) List(pluginID string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	perms, ok := ps.perms[pluginID]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(perms))
	for p := range perms {
		result = append(result, p)
	}
	return result
}
